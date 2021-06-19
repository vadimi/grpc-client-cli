package cliext

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnumValue(t *testing.T) {
	ev := &EnumValue{
		Enum: []string{"v1", "v2"},
	}

	require.NoError(t, ev.Set("v1"))

	assert.Equal(t, "v1", ev.String())
}

func TestEnumDefault(t *testing.T) {
	ev := &EnumValue{
		Enum:    []string{"v1", "v2"},
		Default: "v2",
	}

	assert.Equal(t, "v2", ev.String())
}

func TestEnumError(t *testing.T) {
	ev := &EnumValue{
		Enum:    []string{"v1", "v2"},
		Default: "v2",
	}

	err := ev.Set("invalid")

	assert.Error(t, err)
}
