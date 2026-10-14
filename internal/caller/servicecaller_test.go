package caller

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jhump/protoreflect/v2/protobuilder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vadimi/grpc-client-cli/internal/testing/grpc_testing"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestMarshalJSON(t *testing.T) {
	md, err := protobuilder.NewMessage("any").
		AddField(protobuilder.NewField("id", protobuilder.FieldTypeInt32())).
		AddField(protobuilder.NewField("name", protobuilder.FieldTypeString())).
		Build()
	require.NoError(t, err, "error building new message descriptor")

	m := dynamicpb.NewMessage(md)
	idField := m.Descriptor().Fields().ByName("id")
	m.Set(idField, protoreflect.ValueOfInt32(1))
	nameField := m.Descriptor().Fields().ByName("name")
	m.Set(nameField, protoreflect.ValueOfString("test"))

	sc := NewServiceCaller(nil, JSON, JSON, false)
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

func TestMarshalText(t *testing.T) {
	req := grpc_testing.SimpleRequest{}
	dynReq := dynamicpb.NewMessage(req.ProtoReflect().Descriptor())

	responseStatusField := dynReq.Descriptor().Fields().ByName("response_status")
	rs := &grpc_testing.EchoStatus{}
	dynRS := dynamicpb.NewMessage(rs.ProtoReflect().Descriptor())

	codeField := dynRS.Descriptor().Fields().ByName("code")
	dynRS.Set(codeField, protoreflect.ValueOfInt32(1))
	messageField := dynRS.Descriptor().Fields().ByName("message")
	dynRS.Set(messageField, protoreflect.ValueOfString("oops"))
	dynReq.Set(responseStatusField, protoreflect.ValueOf(dynRS))

	sc := NewServiceCaller(nil, JSON, Text, false)
	res, err := sc.marshalMessage(dynReq)
	require.NoError(t, err)

	assert.Equal(t, "response_status:{code:1 message:\"oops\"}", strings.ReplaceAll(string(res), "  ", " "))
}

func TestMarshalJSON_AnyNotFound(t *testing.T) {
	mdAny := (*anypb.Any)(nil).ProtoReflect().Descriptor()
	md, err := protobuilder.NewMessage("any").
		AddField(protobuilder.NewField("id", protobuilder.FieldTypeInt32())).
		AddField(protobuilder.NewField("name", protobuilder.FieldTypeString())).
		AddField(protobuilder.NewField("a", protobuilder.FieldTypeImportedMessage(mdAny))).
		Build()
	require.NoError(t, err, "error building new message descriptor")

	aValue := &anypb.Any{
		TypeUrl: "test.protobuf.DoesNotExist",
		Value:   []byte("MTIz"),
	}
	m := dynamicpb.NewMessage(md)
	m.Set(m.Descriptor().Fields().ByName("id"), protoreflect.ValueOf(int32(1)))
	m.Set(m.Descriptor().Fields().ByName("name"), protoreflect.ValueOf("test"))
	m.Set(m.Descriptor().Fields().ByName("a"), protoreflect.ValueOfMessage(aValue.ProtoReflect()))

	sc := NewServiceCaller(nil, JSON, JSON, false)
	b, err := sc.marshalMessage(m)
	require.NoError(t, err)

	res := struct {
		ID   int
		Name string
		A    struct {
			TypeURL string `json:"@type"`
			Err     string
		}
	}{}

	err = json.Unmarshal(b, &res)
	require.NoError(t, err)

	assert.Equal(t, 1, res.ID)
	assert.Equal(t, "test", res.Name)
	assert.Equal(t, aValue.TypeUrl, res.A.TypeURL)
	assert.NotEmpty(t, res.A.Err, "err should not be empty")
}
