package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	clitesting "github.com/vadimi/grpc-client-cli/internal/testing/grpc_testing"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func localMessageDesc(t *testing.T) protoreflect.MessageDescriptor {
	t.Helper()
	md := clitesting.File_test_proto.Messages().ByName("SimpleRequest")
	require.NotNil(t, md, "SimpleRequest message not found in the test proto")
	return md
}

func TestValidateMessageJSON(t *testing.T) {
	md := localMessageDesc(t)

	cases := []struct {
		name        string
		msg         string
		errExpected bool
	}{
		{name: "ValidMessage", msg: `{"user": {"id": 1}}`},
		{name: "EmptyObject", msg: `{}`},
		{name: "UnknownField", msg: `{"unknown_field": 1}`, errExpected: true},
		{name: "InvalidSyntax", msg: `{"user": `, errExpected: true},
		{name: "NotAnObject", msg: `nope`, errExpected: true},
		{name: "EmptyMessage", msg: ``, errExpected: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateMessage([]byte(c.msg), md, caller.JSON)
			if c.errExpected {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateMessageText(t *testing.T) {
	md := localMessageDesc(t)

	err := ValidateMessage([]byte(`user { id: 1 }`), md, caller.Text)
	require.NoError(t, err)

	err = ValidateMessage([]byte(`user { id: `), md, caller.Text)
	require.Error(t, err)
}

func TestMessageDefaults(t *testing.T) {
	// message type fields are not emitted; use a message with scalars
	md := clitesting.File_test_proto.Messages().ByName("EchoStatus")
	require.NotNil(t, md)

	assert.Contains(t, MessageDefaults(md), `"code"`)
	assert.Contains(t, MessageDefaults(md), `"message"`)
}

func TestMessageProto(t *testing.T) {
	md := localMessageDesc(t)

	assert.Contains(t, MessageProto(md), "message SimpleRequest")
}

func TestFieldNames(t *testing.T) {
	md := localMessageDesc(t)

	names := FieldNames(md)
	assert.Contains(t, names, "response_status")
	assert.Contains(t, names, "user")
	// nested fields are included for completion
	assert.Contains(t, names, "name")

	// names are sorted
	assert.True(t, isSorted(names), "field names should be sorted: %v", names)
}

func isSorted(names []string) bool {
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			return false
		}
	}
	return true
}

func TestPrintFile(t *testing.T) {
	var w testWriter
	err := PrintFile(&w, clitesting.File_test_proto)
	require.NoError(t, err)
	assert.Contains(t, w.String(), "service TestService")
}

type testWriter struct {
	data []byte
}

func (w *testWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

func (w *testWriter) String() string {
	return string(w.data)
}
