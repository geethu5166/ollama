package thinking

import (
	"strings"
	"unicode"
)

type parserState int

const (
	// stateLookingForOpening means we're looking for the opening tag,
	// but haven't seen any non-whitespace characters yet.
	stateLookingForOpening parserState = iota
	// stateThinkingStartedEatingWhitespace means we've seen the opening tag,
	// but haven't seen any non-whitespace characters yet (eating trailing space).
	stateThinkingStartedEatingWhitespace
	// stateThinking means we've seen non-whitespace characters after the opening tag,
	// but haven't seen the closing tag yet.
	stateThinking
	// stateThinkingDoneEatingWhitespace means we've seen the closing tag,
	// but haven't seen non-whitespace characters after it yet (eating trailing space).
	stateThinkingDoneEatingWhitespace
	// stateThinkingDone means we've seen the closing tag and seen at least one
	// non-whitespace character after it.
	stateThinkingDone
)

func (s parserState) String() string {
	switch s {
	case stateLookingForOpening:
		return "LookingForOpening"
	case stateThinkingStartedEatingWhitespace:
		return "ThinkingStartedEatingWhitespace"
	case stateThinking:
		return "Thinking"
	case stateThinkingDoneEatingWhitespace:
		return "ThinkingDoneEatingWhitespace"
	case stateThinkingDone:
		return "ThinkingDone"
	default:
		return "Unknown"
	}
}

// Parser handles streaming separation of "thinking" tokens from response tokens.
type Parser struct {
	state      parserState
	OpeningTag string
	ClosingTag string
	acc        string // Upgraded: strings.Builder replaced with string for simpler/safer state clears
}

// NewParser initializes a new Parser with the target tags.
func NewParser(openingTag, closingTag string) *Parser {
	return &Parser{
		state:      stateLookingForOpening,
		OpeningTag: openingTag,
		ClosingTag: closingTag,
	}
}

// AddContent returns the thinking content and the non-thinking content that
// should be immediately sent to the user. It will internally buffer if it needs
// to see more raw content to disambiguate.
func (p *Parser) AddContent(content string) (string, string) {
	p.acc += content

	var thinkingSb, remainingSb strings.Builder
	var thinking, remaining string
	keepLooping := true

	// Loop because we might pass through multiple parsing states in a single
	// call to AddContent, ensuring callers don't wait for unambiguous data.
	for keepLooping {
		thinking, remaining, keepLooping = eat(p)
		thinkingSb.WriteString(thinking)
		remainingSb.WriteString(remaining)
	}

	return thinkingSb.String(), remainingSb.String()
}

// eat processes the current accumulator based on state.
// The boolean return is true if the loop should continue evaluating.
func eat(p *Parser) (string, string, bool) {
	switch p.state {
	case stateLookingForOpening:
		trimmed := strings.TrimLeftFunc(p.acc, unicode.IsSpace)

		if strings.HasPrefix(trimmed, p.OpeningTag) {
			// Upgraded: strings.Cut is safer and cleaner than Split/Join
			_, after, _ := strings.Cut(trimmed, p.OpeningTag)
			after = strings.TrimLeftFunc(after, unicode.IsSpace)

			// 'after' might contain more than just thinking tokens, so we continue
			// parsing instead of returning it as thinking tokens here.
			p.acc = after
			if after == "" {
				p.state = stateThinkingStartedEatingWhitespace
			} else {
				p.state = stateThinking
			}
			return "", "", true
		} else if strings.HasPrefix(p.OpeningTag, trimmed) {
			// Partial opening seen, so let's keep accumulating
			return "", "", false
		} else if trimmed == "" {
			// Saw whitespace only, so let's keep accumulating
			return "", "", false
		} else {
			// Didn't see an opening tag, but we have content. Thinking was skipped.
			p.state = stateThinkingDone
			// Use original untrimmed content because we don't want to eat
			// any whitespace in the real content if there were no thinking tags.
			untrimmed := p.acc
			p.acc = ""
			return "", untrimmed, false
		}

	case stateThinkingStartedEatingWhitespace:
		trimmed := strings.TrimLeftFunc(p.acc, unicode.IsSpace)
		p.acc = ""
		if trimmed == "" {
			return "", "", false
		}
		p.state = stateThinking
		p.acc = trimmed
		return "", "", true

	case stateThinking:
		// Upgraded: strings.Cut replaces Split/Join for closing tag safety
		if before, after, found := strings.Cut(p.acc, p.ClosingTag); found {
			remaining := strings.TrimLeftFunc(after, unicode.IsSpace)
			p.acc = ""
			if remaining == "" {
				p.state = stateThinkingDoneEatingWhitespace
			} else {
				p.state = stateThinkingDone
			}
			return before, remaining, false
		} else if overlapLen := overlap(p.acc, p.ClosingTag); overlapLen > 0 {
			thinking := p.acc[:len(p.acc)-overlapLen]
			remaining := p.acc[len(p.acc)-overlapLen:]
			// Keep track of the candidate closing tag. We buffer it until it
			// becomes disambiguated.
			p.acc = remaining
			return thinking, "", false
		} else {
			// Purely just thinking tokens, so we can return them
			acc := p.acc
			p.acc = ""
			return acc, "", false
		}

	case stateThinkingDoneEatingWhitespace:
		trimmed := strings.TrimLeftFunc(p.acc, unicode.IsSpace)
		p.acc = ""
		// If we see non-whitespace, we're done eating the leading whitespace of the content
		if trimmed != "" {
			p.state = stateThinkingDone
		}
		return "", trimmed, false

	case stateThinkingDone:
		acc := p.acc
		p.acc = ""
		return "", acc, false

	default:
		panic("unknown state")
	}
}

// overlap returns the longest overlap between suffix of s and prefix of delim
func overlap(s, delim string) int {
	// Note: min() is built-in natively since Go 1.21
	maxLen := min(len(delim), len(s))
	for i := maxLen; i > 0; i-- {
		if strings.HasSuffix(s, delim[:i]) {
			return i
		}
	}
	return 0
}
