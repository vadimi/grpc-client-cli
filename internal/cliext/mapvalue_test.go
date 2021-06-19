package cliext

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapValue(t *testing.T) {
	mv := NewMapValue()
	require.NoError(t, mv.Set("a: b"))
	require.NoError(t, mv.Set("c: d"))

	res := ParseMapValue(mv)

	assert.Equal(t, []string{"b"}, res["a"])
	assert.Equal(t, []string{"d"}, res["c"])
}

func TestMapValueMulti(t *testing.T) {
	mv := NewMapValue()
	require.NoError(t, mv.Set("a: b"))
	require.NoError(t, mv.Set("a: d"))

	res := ParseMapValue(mv)

	assert.Equal(t, []string{"b", "d"}, res["a"])
}

func TestMapValueSerialize(t *testing.T) {
	mv := NewMapValue()
	require.NoError(t, mv.Set("a: b"))
	require.NoError(t, mv.Set("c: d"))

	serialied := mv.Serialize()

	require.NoError(t, mv.Set(serialied))

	res := ParseMapValue(mv)

	assert.Equal(t, []string{"b"}, res["a"])
	assert.Equal(t, []string{"d"}, res["c"])
}

func TestMapValueError(t *testing.T) {
	tests := []string{
		"invalid", "  :  invalid", "invalid: ",
	}

	for _, tt := range tests {
		mv := NewMapValue()
		err := mv.Set(tt)

		assert.Error(t, err, tt)
	}
}
