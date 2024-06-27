package resp

import (
	"testing"
)

func TestTranslatePattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		input       string
		shouldMatch bool
	}{
		{"Basic wildcard", "h*llo", "hello", true},
		{"Wildcard middle", "h*llo", "heeeello", true},
		{"Single character", "h?llo", "hallo", true},
		{"Character class", "h[ae]llo", "hello", true},
		{"Negation in character class", "h[^e]llo", "hallo", true},
		{"Range in character class", "h[a-b]llo", "hallo", true},
		{"Escaped wildcard", "h*llo\\*", "hello*", true},
		{"Escaped wildcard no match", "h*llo\\*", "hello", false},
		{"Escaped Special Char", "h\\[e\\]llo", "h[e]llo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := TranslatePattern(tt.pattern)
			if err != nil {
				t.Fatalf("TranslatePattern() error = %v", err)
			}

			matched := re.MatchString(tt.input)
			if matched != tt.shouldMatch {
				t.Errorf("TranslatePattern() = %v, want %v for pattern %s and input %s", matched, tt.shouldMatch, tt.pattern, tt.input)
			}
		})
	}
}
