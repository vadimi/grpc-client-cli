package caller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToLowerCamelCase(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{name: "no_transform", text: "testString", expected: "testString"},
		{name: "mixed_case", text: "test_Str", expected: "testStr"},
		{name: "lower_case", text: "test_Str", expected: "testStr"},
		{name: "multiple_occurances", text: "test_Str_str", expected: "testStrStr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := toLowerCamelCase(tt.text)
			assert.Equal(t, tt.expected, res)
		})
	}
}
