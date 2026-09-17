package app

import (
	"io"
	"regexp"

	"charm.land/lipgloss/v2"
)

// jsonToken matches the JSON tokens that get colored: object keys, string
// values, numbers and literals.
var jsonToken = regexp.MustCompile(`"(?:[^"\\]|\\.)*"(\s*:)?|-?\b\d+(?:\.\d+)?(?:[eE][+-]?\d+)?\b|\b(?:true|false|null)\b`)

// highlighter colors JSON messages for display.
type highlighter struct {
	key, str, num, lit lipgloss.Style
}

func newHighlighter(isDark bool) highlighter {
	c := lipgloss.LightDark(isDark)
	return highlighter{
		key: lipgloss.NewStyle().Foreground(c(lipgloss.Color("#0b6e99"), lipgloss.Color("#7dcfff"))),
		str: lipgloss.NewStyle().Foreground(c(lipgloss.Color("#2f7d32"), lipgloss.Color("#9ece6a"))),
		num: lipgloss.NewStyle().Foreground(c(lipgloss.Color("#b45f06"), lipgloss.Color("#ff9e64"))),
		lit: lipgloss.NewStyle().Foreground(c(lipgloss.Color("#8e24aa"), lipgloss.Color("#bb9af7"))),
	}
}

// json returns b with its JSON tokens colored.
func (h highlighter) json(b []byte) []byte {
	return jsonToken.ReplaceAllFunc(b, func(tok []byte) []byte {
		s := string(tok)
		switch {
		case s[0] == '"' && s[len(s)-1] == ':':
			// keep the colon and the whitespace before it unstyled
			end := len(s) - 1
			for end > 0 && s[end-1] != '"' {
				end--
			}
			return []byte(h.key.Render(s[:end]) + s[end:])
		case s[0] == '"':
			return []byte(h.str.Render(s))
		case s == "true" || s == "false" || s == "null":
			return []byte(h.lit.Render(s))
		default:
			return []byte(h.num.Render(s))
		}
	})
}

// highlightWriter writes the plain output to plain and a colored copy to
// styled.
type highlightWriter struct {
	plain, styled io.Writer
	hl            *highlighter
}

func (w highlightWriter) Write(b []byte) (int, error) {
	if _, err := w.plain.Write(b); err != nil {
		return 0, err
	}
	out := b
	if w.hl != nil {
		out = w.hl.json(b)
	}
	if _, err := w.styled.Write(out); err != nil {
		return 0, err
	}
	return len(b), nil
}
