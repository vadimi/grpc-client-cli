package caller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestResolver_WellKnown(t *testing.T) {
	r := newResolver()

	typeName := "google.protobuf.StringValue"
	m, err := r.FindMessageByName(protoreflect.FullName(typeName))
	require.NoError(t, err)

	fullName := string(m.Descriptor().FullName())
	require.Equal(t, typeName, fullName)
}

func TestResolver_LoadedFiles(t *testing.T) {
	sml := NewServiceMetadataProto([]string{"../../testdata/test.proto"}, nil)
	_, err := sml.GetServiceMetaDataList(context.Background())
	require.NoError(t, err)

	r := newResolver()

	userType := "grpc_client_cli.testing.User"

	typeURL := "type.googleapis.com/" + userType
	m, err := r.FindMessageByURL(typeURL)
	require.NoError(t, err)

	require.NoError(t, err)
	require.Equal(t, userType, string(m.Descriptor().FullName()))
}

func TestResolver_UnmarshalEmpty(t *testing.T) {
	r := newResolver()
	m, err := r.FindMessageByURL("testing.protobuf.DoesNotExist")
	require.NoError(t, err)
	require.Equal(t, "any", string(m.Descriptor().FullName()))
	msg := dynamicpb.NewMessage(m.Descriptor())
	res, err := protojson.Marshal(msg)
	require.NoError(t, err)
	require.Equal(t, "{}", string(res))
}
