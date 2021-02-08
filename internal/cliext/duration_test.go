package cliext

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDurationParse(t *testing.T) {
	tests := []struct {
		name      string
		val       string
		expected  time.Duration
		expectErr bool
	}{
		{name: "parse empty", val: "", expectErr: true},
		{name: "parse invalid", val: "oops", expectErr: true},
		{name: "parse invalid format", val: "1hour", expectErr: true},
		{name: "parse seconds", val: "15s", expected: 15 * time.Second},
		{name: "parse default seconds", val: "5", expected: 5 * time.Second},
		{name: "parse minutes", val: "5m", expected: 5 * time.Minute},
		{name: "parse mixed", val: "1m10s", expected: 70 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ParseDuration(tt.val)

			if tt.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tt.expected, res)
			}
		})
	}
}
