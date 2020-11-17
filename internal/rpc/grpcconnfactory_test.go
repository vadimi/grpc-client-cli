package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithAuthority(t *testing.T) {
	authority := "authority1"
	grpcConnFact := NewGrpcConnFactory(WithAuthority(authority))

	assert.Equal(t, authority, grpcConnFact.settings.authority)
}

func TestMergeMetadata(t *testing.T) {
	grpcConnFact := NewGrpcConnFactory(WithHeaders(map[string][]string{
		"header1": {"val1"},
	}))

	moreHaders := map[string][]string{
		"header2": {"val2"},
		"header1": {"val3"},
	}

	res := grpcConnFact.metadata(moreHaders)

	assert.Equal(t, []string{"val1", "val3"}, res["header1"])
	assert.Equal(t, []string{"val2"}, res["header2"])
}
