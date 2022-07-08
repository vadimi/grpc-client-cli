package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/spyzhov/ajson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	app_testing "github.com/vadimi/grpc-client-cli/internal/testing"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestMain(m *testing.M) {
	err := app_testing.SetupTestServer()
	if err != nil {
		panic(err)
	}

	defer app_testing.StopTestServer()
	os.Exit(m.Run())
}

func TestAppServiceCalls(t *testing.T) {
	runAppServiceCalls(t, &startOpts{
		Target:        app_testing.TestServerAddr(),
		Deadline:      15,
		IsInteractive: false,
	})
}

func TestAppWellKnownAnyServiceCall(t *testing.T) {
	appOpts := &startOpts{
		Target:        app_testing.TestServerAddr(),
		Deadline:      15,
		IsInteractive: false,
	}
	buf := &bytes.Buffer{}
	appOpts.w = buf
	appOpts.ProtoImports = []string{"../../testdata/user_props.proto"}
	app, err := newApp(appOpts)
	require.NoError(t, err)

	m, ok := findMethod(t, app, "grpc_client_cli.testing.TestService", "UnaryAny")
	if !ok {
		return
	}
	msgTmpl := `
{
  "user_id": 1,
  "user_props": {
    "@type": "type.googleapis.com/grpc_client_cli.testing.UserProps",
    "name": "test"
  }
}
`
	msg := []byte(msgTmpl)

	err = app.callClientStream(context.Background(), m, [][]byte{msg})
	require.NoError(t, err, "error executing callClientStream()")

	res := buf.Bytes()
	root, err := ajson.Unmarshal(res)
	require.NoError(t, err, "error unmarshaling result json")

	require.Equal(t, int32(1), jsonInt32(root, "$.user_id"), "user_id not found")
	require.Equal(t, "test", jsonString(root, "$.user_props.name"), "user_props.name not found")
	require.Equal(t, "type.googleapis.com/grpc_client_cli.testing.UserProps", jsonString(root, "$.user_props.@type"), "user_props.@type not found")
}

func runAppServiceCalls(t *testing.T, appOpts *startOpts) {
	buf := &bytes.Buffer{}
	appOpts.w = buf
	app, err := newApp(appOpts)
	if err != nil {
		t.Error(err)
		return
	}

	t.Run("appCallUnaryServerError", func(t *testing.T) {
		appCallUnaryServerError(t, app)
	})

	t.Run("appCallUnary", func(t *testing.T) {
		buf.Reset()
		appCallUnary(t, app, buf)
	})

	t.Run("appCallStreamOutput", func(t *testing.T) {
		buf.Reset()
		appCallStreamOutput(t, app, buf)
	})

	t.Run("appCallStreamOutputError", func(t *testing.T) {
		appCallStreamOutputError(t, app)
	})

	t.Run("appCallClientStream", func(t *testing.T) {
		buf.Reset()
		appCallClientStream(t, app, buf)
	})

	t.Run("appCallClientStreamError", func(t *testing.T) {
		buf.Reset()
		appCallClientStreamError(t, app)
	})

	t.Run("appCallFullDuplexBidiStream", func(t *testing.T) {
		buf.Reset()
		appCallBidiStream(t, app, buf, "FullDuplexCall")
	})

	t.Run("appCallFullDuplexBidiStreamError", func(t *testing.T) {
		buf.Reset()
		appCallBidiStreamError(t, app, buf)
	})

	t.Run("appCallFullDuplexBidiStreamErrorProcessing", func(t *testing.T) {
		buf.Reset()
		appCallBidiStreamErrorProcessing(t, app, buf)
	})

	t.Run("appCallHalfDuplexBidiStream", func(t *testing.T) {
		buf.Reset()
		appCallBidiStream(t, app, buf, "HalfDuplexCall")
	})
}

func appCallUnaryServerError(t *testing.T, app *app) {
	m, ok := findMethod(t, app, "grpc_client_cli.testing.TestService", "UnaryCall")
	if !ok {
		return
	}

	errCode := int32(codes.Internal)

	msgTmpl := `
{
  "response_status": {
    "code": %d
  }
}
`

	msg := []byte(fmt.Sprintf(msgTmpl, errCode))

	err := app.callClientStream(context.Background(), m, [][]byte{msg})
	if err == nil {
		t.Error("error expected, got nil")
		return
	}

	s, _ := status.FromError(errors.Unwrap(err))
	if s.Code() != codes.Code(errCode) {
		t.Errorf("expectd status code %v, got %v, err: %v", codes.Code(errCode), s.Code(), err)
	}
}

func appCallUnary(t *testing.T, app *app, buf *bytes.Buffer) {
	m, ok := findMethod(t, app, "grpc_client_cli.testing.TestService", "UnaryCall")
	if !ok {
		return
	}

	userId := int32(123)
	userName := "testuser"

	msgTmpl := `
{
  "user": {"id": %d, "name": "%s"}
}
`
	msg := []byte(fmt.Sprintf(msgTmpl, userId, userName))

	if app.opts.InFormat == caller.Text {
		msgTmpl = `user { id: %d name: "%s" }`
		msg = []byte(fmt.Sprintf(msgTmpl, userId, userName))
	}

	err := app.callClientStream(context.Background(), m, [][]byte{msg})
	if err != nil {
		t.Errorf("error executing callClientStream(): %v", err)
		return
	}

	res := buf.Bytes()
	root, err := ajson.Unmarshal(res)
	if err != nil {
		t.Errorf("error unmarshaling result json: %v", err)
		return
	}

	if jsonInt32(root, "$.user.id") != userId {
		t.Errorf("payload type not found: %s", res)
		return
	}

	if jsonString(root, "$.user.name") != userName {
		t.Errorf("payload body not found: %s, %s", res, userName)
	}
}

func appCallStreamOutput(t *testing.T, app *app, buf *bytes.Buffer) {
	m, ok := findMethod(t, app, "grpc_client_cli.testing.TestService", "StreamingOutputCall")
	if !ok {
		return
	}

	respSize1 := 3
	respSize2 := 5
	userName := "testuser"

	msgTmpl := `
{
  "user": {
	"name": "%s"
  },
  "response_parameters": [{
	"size": %d
  },{
	"size": %d
  }]
}
`

	getEncBody := func(c int) string {
		body := strings.Repeat(userName, c)
		return body
	}

	msg := []byte(fmt.Sprintf(msgTmpl, userName, respSize1, respSize2))

	if app.opts.InFormat == caller.Text {
		msgTmpl = `user { name: "%s"} response_parameters: {size: %d} response_parameters: {size: %d}`
		msg = []byte(fmt.Sprintf(msgTmpl, userName, respSize1, respSize2))
	}

	err := app.callStream(context.Background(), m, [][]byte{msg})
	require.NoError(t, err, "error executing callStream()")

	res := buf.Bytes()
	root, err := ajson.Unmarshal(res)
	require.NoError(t, err, "error unmarshaling result json")

	if len(root.MustArray()) < 2 {
		t.Fatalf("expected %d elements", 2)
	}

	assert.Equal(t, jsonString(root, "$[0].user.name"), getEncBody(respSize1))
	assert.Equal(t, jsonString(root, "$[1].user.name"), getEncBody(respSize2))
}

func appCallBidiStreamErrorProcessing(t *testing.T, app *app, buf *bytes.Buffer) {
	m, ok := findMethod(t, app, "grpc_client_cli.testing.TestService", "FullDuplexCall")
	if !ok {
		return
	}

	errCode := int32(codes.Aborted)
	respSize1 := 3
	respSize2 := 5
	userName := "testuser"

	msgTmpl := `
{
  "user": {
    "name": "%s"
  },
  "response_parameters": [{
    "size": %d
  },{
    "size": %d
  }]
}
`

	msgTmplErr := `
{
  "response_status": {
    "code": %d
  }
}
`

	getEncBody := func(c int) string {
		return strings.Repeat(userName, c)
	}

	msg := []byte(fmt.Sprintf(msgTmpl, getEncBody(1), respSize1, respSize2))
	msgErr := []byte(fmt.Sprintf(msgTmplErr, errCode))

	messages := [][]byte{msg, msgErr}
	err := app.callStream(context.Background(), m, messages)
	if err == nil {
		t.Fatal("error expected, got nil")
	}

	s, _ := status.FromError(errors.Unwrap(err))
	if s.Code() != codes.Code(errCode) {
		t.Fatalf("expected status code %v, got %v", codes.Code(errCode), s.Code())
	}

	res := buf.Bytes()
	root, err := ajson.Unmarshal(res)
	if err != nil {
		t.Fatalf("error unmarshaling result json: %v", err)
	}

	if len(root.MustArray()) < 2 {
		t.Fatalf("expected %d elements, got %d", 2, len(root.MustArray()))
	}
}

func appCallBidiStreamError(t *testing.T, app *app, buf *bytes.Buffer) {
	m, ok := findMethod(t, app, "grpc_client_cli.testing.TestService", "FullDuplexCall")
	if !ok {
		return
	}

	errCode := int32(codes.Internal)
	respSize1 := 3
	respSize2 := 5
	userName := "testuser"

	msgTmpl := `
{
  "user": {
    "name": "%s"
  },
  "response_parameters": [{
    "size": %d
  },{
    "size": %d
  }]
}
`

	getEncBody := func(c int) string {
		return strings.Repeat(userName, c)
	}

	msg := []byte(fmt.Sprintf(msgTmpl, getEncBody(1), respSize1, respSize2))

	ctx := metadata.AppendToOutgoingContext(context.Background(), app_testing.MethodExitCode, fmt.Sprintf("%d", errCode))
	messages := [][]byte{msg, msg}
	err := app.callStream(ctx, m, messages)
	if err == nil {
		t.Error("error expected, got nil")
		return
	}

	s, _ := status.FromError(errors.Unwrap(err))
	if s.Code() != codes.Code(errCode) {
		t.Errorf("expectd status code %v, got %v, err: %v", codes.Code(errCode), s.Code(), err)
		return
	}

	resp := strings.TrimSpace(buf.String())
	if resp != "[]" {
		t.Errorf("expected `[]` response, got %s", resp)
	}
}

func appCallBidiStream(t *testing.T, app *app, buf *bytes.Buffer, methodName string) {
	m, ok := findMethod(t, app, "grpc_client_cli.testing.TestService", methodName)
	if !ok {
		return
	}

	respSize1 := 3
	respSize2 := 5
	userName := "testuser"

	msgTmpl := `
{
  "user": {
    "name": "%s"
  },
  "response_parameters": [{
    "size": %d
  },{
    "size": %d
  }]
}
`

	getEncBody := func(c int) string {
		return strings.Repeat(userName, c)
	}

	msg := []byte(fmt.Sprintf(msgTmpl, getEncBody(1), respSize1, respSize2))

	messages := [][]byte{msg, msg}
	err := app.callStream(context.Background(), m, messages)
	if err != nil {
		t.Errorf("error executing callStream(): %v", err)
		return
	}

	res := buf.Bytes()
	root, err := ajson.Unmarshal(res)
	if err != nil {
		t.Errorf("error unmarshaling result json: %v", err)
		return
	}

	if len(root.MustArray()) < len(messages)*2 {
		t.Errorf("expected %d elements, got %d", len(messages)*2, len(root.MustArray()))
		return
	}

	for i := 0; i < len(messages)*2; i++ {
		body := jsonString(root, fmt.Sprintf("$[%d].user.name", i))
		expected := getEncBody(respSize2)
		if i%2 == 0 {
			expected = getEncBody(respSize1)
		}
		if body != expected {
			t.Errorf("payload body[%d] not found: %s", i, root)
			return
		}
	}
}

func appCallStreamOutputError(t *testing.T, app *app) {
	m, ok := findMethod(t, app, "grpc_client_cli.testing.TestService", "StreamingOutputCall")
	if !ok {
		return
	}

	errCode := int32(codes.Internal)

	msgTmpl := `
{
  "response_status": {
    "code": %d
  }
}
`

	msg := fmt.Sprintf(msgTmpl, errCode)

	err := app.callStream(context.Background(), m, [][]byte{[]byte(msg)})
	if err == nil {
		t.Error("error expected, got nil")
		return
	}

	s, _ := status.FromError(errors.Unwrap(err))
	if s.Code() != codes.Code(errCode) {
		t.Errorf("expectd status code %v, got %v", codes.Code(errCode), s.Code())
	}
}

func appCallClientStreamError(t *testing.T, app *app) {
	m, ok := findMethod(t, app, "grpc_client_cli.testing.TestService", "StreamingInputCall")
	if !ok {
		return
	}

	errCode := int32(codes.Internal)

	userName := "testuser"

	msgTmpl := `
[
  {
    "user": {
      "name": "%s"
    }
  }
]
`

	msg := fmt.Sprintf(msgTmpl, userName)
	msgArr, err := toJSONArray([]byte(msg))
	if err != nil {
		t.Error(err)
		return
	}

	ctx := metadata.AppendToOutgoingContext(context.Background(), app_testing.MethodExitCode, fmt.Sprintf("%d", errCode))

	err = app.callClientStream(ctx, m, msgArr)
	if err == nil {
		t.Error("error expected, got nil")
		return
	}

	s, _ := status.FromError(errors.Unwrap(err))
	if s.Code() != codes.Code(errCode) {
		t.Errorf("expectd status code %v, got %v", codes.Code(errCode), s.Code())
	}
}

func appCallClientStream(t *testing.T, app *app, buf *bytes.Buffer) {
	m, ok := findMethod(t, app, "grpc_client_cli.testing.TestService", "StreamingInputCall")
	if !ok {
		return
	}

	userName := "testuser"

	msgTmpl := `
[
  {
    "user": {
      "name": "%s"
    }
  },
  {
    "user": {
      "name": "%s"
    }
  }
]
`

	msg := fmt.Sprintf(msgTmpl, userName, userName)
	msgArr, err := toJSONArray([]byte(msg))
	if err != nil {
		t.Error(err)
		return
	}

	err = app.callClientStream(context.Background(), m, msgArr)
	if err != nil {
		t.Errorf("error executing callClientStream(): %v", err)
		return
	}

	res := buf.Bytes()
	root, err := ajson.Unmarshal(res)
	if err != nil {
		t.Errorf("error unmarshaling result json: %v", err)
		return
	}

	if jsonInt32(root, "$.aggregated_payload_size") != int32(len(userName)*2) {
		t.Errorf("aggregated_payload_size is invalid: %s", res)
		return
	}
}

func jsonInt32(n *ajson.Node, jsonPath string) int32 {
	nodes, err := n.JSONPath(jsonPath)
	if err != nil {
		panic(err)
	}

	return int32(nodes[0].MustNumeric())
}

func jsonString(n *ajson.Node, jsonPath string) string {
	nodes, err := n.JSONPath(jsonPath)
	if err != nil {
		panic(err)
	}

	return nodes[0].MustString()
}

func findMethod(t *testing.T, app *app, serviceName, methodName string) (*desc.MethodDescriptor, bool) {
	m, err := app.selectMethod(app.getService(serviceName), methodName)
	if err != nil {
		t.Error(err)
		return nil, false
	}

	if m == nil {
		t.Error("method not found")
		return nil, false
	}

	return m, true
}

func TestStatsHandler(t *testing.T) {
	buf := &bytes.Buffer{}
	app, err := newApp(&startOpts{
		Target:        app_testing.TestServerAddr(),
		Deadline:      15,
		IsInteractive: false,
		Verbose:       true,
		w:             buf,
		Headers: map[string][]string{
			"test":       {"v1"},
			"test_multi": {"a1", "a2"},
		},
	})
	if err != nil {
		t.Error(err)
		return
	}

	userId := int32(123)
	userName := "testuser"

	msgTmpl := `
{
  "user": { "id": %d, "name": "%s" }
}
`

	msg := []byte(fmt.Sprintf(msgTmpl, userId, userName))

	t.Run("checkStats", func(t *testing.T) {
		checkStats(t, app, msg)
	})

	t.Run("checkStatsInOutput", func(t *testing.T) {
		checkStatsInOutput(t, app, msg, buf)
	})
}

func TestToJSONArrayCoversion(t *testing.T) {
	cases := []struct {
		name        string
		msg         string
		msgCount    int
		errExpected bool
	}{
		{name: "OneMessage", msg: `[{"name": "str"}]`, msgCount: 1, errExpected: false},
		{name: "OneMessageNoArraySyntax", msg: `{"name": "str"}`, msgCount: 1, errExpected: false},
		{name: "OneMessageNoArraySyntaxWhiteSpaces", msg: `
{"name": "str"}
`, msgCount: 1, errExpected: false},
		{name: "MultipleMessages", msg: `[{"name": "str1"},{"name": "str2"}]`, msgCount: 2, errExpected: false},
		{name: "InvalidSyntax", msg: `[{"name": "str1"}`, errExpected: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := toJSONArray([]byte(c.msg))
			if c.errExpected && err == nil {
				t.Error("json error expected, got nil")
				return
			}
			if !c.errExpected && err != nil {
				t.Errorf("no json error expected, got %v", err)
				return
			}

			if len(res) != c.msgCount {
				t.Errorf("expected %d messages, got %d", c.msgCount, len(res))
			}
		})
	}
}

func TestAuthorityHeader(t *testing.T) {
	authority1 := "testservice1"
	authority2 := "testservice2"
	tests := []struct {
		name              string
		authority         string
		target            string
		expectedAuthority string
	}{
		{
			name:              "defaultAuthority",
			target:            app_testing.TestServerAddr(),
			expectedAuthority: app_testing.TestServerAddr(),
		},
		{
			name:              "customAuthorityInTarget",
			target:            app_testing.TestServerAddr() + ",authority=" + authority1,
			expectedAuthority: authority1,
		},
		{
			name:              "customAuthorityArg",
			target:            app_testing.TestServerAddr() + ",authority=" + authority1,
			authority:         authority2,
			expectedAuthority: authority2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}

			app, err := newApp(&startOpts{
				Target:        tt.target,
				Deadline:      15,
				Authority:     tt.authority,
				IsInteractive: false,
				w:             buf,
			})
			if err != nil {
				t.Fatal(err)
			}

			m, ok := findMethod(t, app, "grpc_client_cli.testing.TestService", "UnaryCall")
			if !ok {
				return
			}

			userId := int32(123)
			userName := "testuser"

			msgTmpl := `
{
  "user": { "id": %d, "name": "%s" }
}
`

			msg := []byte(fmt.Sprintf(msgTmpl, userId, userName))

			ctx := metadata.AppendToOutgoingContext(context.Background(), app_testing.CheckHeader, ":authority="+tt.expectedAuthority)

			err = app.callClientStream(ctx, m, [][]byte{msg})
			require.NoError(t, err, "error executing callClientStream()")
		})
	}
}

func checkStats(t *testing.T, app *app, msg []byte) {
	m, ok := findMethod(t, app, "grpc_client_cli.testing.TestService", "UnaryCall")
	if !ok {
		return
	}

	callTimeout := time.Duration(app.opts.Deadline) * time.Second
	ctx, cancel := context.WithTimeout(rpc.WithStatsCtx(context.Background()), callTimeout)
	defer cancel()

	err := app.callClientStream(ctx, m, [][]byte{msg})
	require.NoError(t, err)

	s := rpc.ExtractRpcStats(ctx)
	require.NotNil(t, s, "stats are missing in ctx")
	assert.NotEmpty(t, s.ReqHeaders())
	assert.Equal(t, []string{"v1"}, s.ReqHeaders()["test"])
	assert.Equal(t, []string{"a1", "a2"}, s.ReqHeaders()["test_multi"])
	assert.NotEmpty(t, s.RespHeaders())
	assert.Equal(t, "/grpc_client_cli.testing.TestService/UnaryCall", s.FullMethod())
	assert.True(t, s.ReqSize() <= s.RespSize(), "ReqSize should be <= RespSize: %v", s)
}

func checkStatsInOutput(t *testing.T, app *app, msg []byte, buf *bytes.Buffer) {
	m, ok := findMethod(t, app, "grpc_client_cli.testing.TestService", "UnaryCall")
	if !ok {
		return
	}

	err := app.callService(m, msg)
	require.NoError(t, err)

	res := buf.String()

	expected := []string{
		"Request duration:", "Request size:", "Response size:",
		"Status:", "Request Headers:", "Response Headers:",
		"Method:",
	}

	for _, e := range expected {
		assert.Contains(t, res, e)
	}
}
