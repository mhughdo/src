package resp

import (
	"fmt"
	"regexp"
	"strings"
)

// TranslatePattern translates a glob-style pattern to a regular expression,
// correctly handling escape sequences.
func TranslatePattern(pattern string) (*regexp.Regexp, error) {
	var rePattern strings.Builder

	rePattern.WriteString("^")

	inEscape := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if inEscape {
			// After a backslash, treat the next character as a literal
			rePattern.WriteString(regexp.QuoteMeta(string(c)))
			inEscape = false
		} else {
			switch c {
			case '\\':
				// Start escape sequence
				inEscape = true
			case '*':
				rePattern.WriteString(".*")
			case '?':
				rePattern.WriteString(".")
			case '[':
				rePattern.WriteString("[")
			case ']':
				rePattern.WriteString("]")
			case '^':
				// Handle the beginning of set negation in character classes
				rePattern.WriteString("^")
			case '-':
				rePattern.WriteString("-")
			default:
				rePattern.WriteString(regexp.QuoteMeta(string(c)))
			}
		}
	}

	if inEscape {
		return nil, fmt.Errorf("invalid escape sequence at end of pattern")
	}

	rePattern.WriteString("$")
	return regexp.Compile(rePattern.String())
}
