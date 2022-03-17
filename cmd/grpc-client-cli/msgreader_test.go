package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWordCompleter(t *testing.T) {
	r := &msgReader{}
	tests := []struct {
		name          string
		completion    string
		expectedCompl string
		expectedHead  string
		expectedTail  string
		words         []string
		pos           int
	}{
		{
			name:          "1",
			words:         []string{"first", "last"},
			completion:    `{"fi`,
			pos:           3,
			expectedCompl: "first",
			expectedHead:  `{"`,
			expectedTail:  "i",
		},
		{
			name:          "2",
			words:         []string{"first", "last"},
			completion:    `{"la`,
			pos:           3,
			expectedCompl: "last",
			expectedHead:  `{"`,
			expectedTail:  "a",
		},
		{
			name:          "3",
			words:         []string{"first", "last"},
			completion:    `{"first": "123", "la"`,
			pos:           19,
			expectedCompl: "last",
			expectedHead:  `{"first": "123", "`,
			expectedTail:  `a"`,
		},
		{
			name:          "case-insensitive",
			words:         []string{"first", "Middle", "last"},
			completion:    `{"Mi`,
			pos:           4,
			expectedCompl: "Middle",
			expectedHead:  `{"`,
			expectedTail:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completer := r.wordCompleter(tt.words)
			head, completions, tail := completer(tt.completion, tt.pos)

			assert.Contains(t, completions, tt.expectedCompl)
			assert.Equal(t, tt.expectedHead, head)
			assert.Equal(t, tt.expectedTail, tail)
		})
	}
}
