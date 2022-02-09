package caller

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/builder"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalJSON(t *testing.T) {
	md, err := builder.NewMessage("any").
		AddField(builder.NewField("id", builder.FieldTypeInt32())).
		AddField(builder.NewField("name", builder.FieldTypeString())).
		Build()
	require.NoError(t, err, "error building new message descriptor")

	m := dynamic.NewMessage(md)
	m.SetFieldByName("id", int32(1))
	m.SetFieldByName("name", "test")

	sc := NewServiceCaller(nil, JSON, JSON, nil)
	b, err := sc.marshalMessage(m)
	require.NoError(t, err)

	res := struct {
		ID   int
		Name string
	}{}

	err = json.Unmarshal(b, &res)
	require.NoError(t, err)

	assert.Equal(t, 1, res.ID)
	assert.Equal(t, "test", res.Name)
}

func TestMarshalJSON_AnyNotFound(t *testing.T) {
	mdAny, err := desc.LoadMessageDescriptorForMessage((*any.Any)(nil))
	require.NoError(t, err, "failed to load Any message descriptor")
	md, err := builder.NewMessage("any").
		AddField(builder.NewField("id", builder.FieldTypeInt32())).
		AddField(builder.NewField("name", builder.FieldTypeString())).
		AddField(builder.NewField("a", builder.FieldTypeImportedMessage(mdAny))).
		Build()
	require.NoError(t, err, "error building new message descriptor")

	aValue := &any.Any{
		TypeUrl: "test.protobuf.DoesNotExist",
		Value:   []byte{'1', '2', '3'},
	}
	m := dynamic.NewMessage(md)
	m.SetFieldByName("id", int32(1))
	m.SetFieldByName("name", "test")
	m.SetFieldByName("a", aValue)

	sc := NewServiceCaller(nil, JSON, JSON, nil)
	b, err := sc.marshalMessage(m)
	require.NoError(t, err)

	res := struct {
		ID   int
		Name string
		A    struct {
			TypeURL string `json:"@type"`
			Value   string
			Err     string
		}
	}{}

	err = json.Unmarshal(b, &res)
	require.NoError(t, err)

	assert.Equal(t, 1, res.ID)
	assert.Equal(t, "test", res.Name)
	assert.Equal(t, base64.StdEncoding.EncodeToString(aValue.Value), res.A.Value)
	assert.Equal(t, aValue.TypeUrl, res.A.TypeURL)
	assert.NotEmpty(t, res.A.Err, "err should not be empty")
}
