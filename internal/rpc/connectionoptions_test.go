package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionOptionsParseHost(t *testing.T) {
	const host = "test1.app"
	tests := []string{host, "host=" + host}

	for _, test := range tests {
		opts, _ := NewConnectionOpts(test)

		require.Equal(t, host, opts.Host)
	}
}

func TestConnectionOptionsParseProxy(t *testing.T) {
	const proxy = "testproxy.app"
	tests := []string{"host=test1.app,authority=" + proxy, "test1.app,authority=" + proxy}

	for _, test := range tests {
		opts, _ := NewConnectionOpts(test)

		require.Equal(t, proxy, opts.Authority)
		require.Equal(t, "test1.app", opts.Host)
	}
}

func TestConnectionOptionsParseError(t *testing.T) {
	_, err := NewConnectionOpts("")
	assert.Error(t, err, "Expected non nil error")
}

func TestConnectionOptionsParseMetadata(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test,metadata=key1:", ""},
		{"test,metadata=key1:value1", "value1"},
		{"test,metadata=key1:value1:value2", "value1:value2"},
		{"test,metadata=key1:value1,metadata=key2:value2", "value1"},
		{"test,metadata=key1:value1,metadata=key1:value2", "value1"},
	}

	for _, test := range tests {
		opts, _ := NewConnectionOpts(test.input)

		val := opts.Metadata["key1"][0]
		assert.Equal(t, test.expected, val)
	}
}
