package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"
	"github.com/spyzhov/ajson"
	app_testing "github.com/vadimi/grpc-client-cli/internal/testing"
	"google.golang.org/grpc/codes"
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
	app, err := newApp(&startOpts{
		Target:        app_testing.TestServerAddr(),
		Deadline:      15,
		IsInteractive: false,
	})

	buf := &bytes.Buffer{}
	app.w = buf

	if err != nil {
		t.Error(err)
		return
	}

	t.Run("appCallUnaryServerError", func(t *testing.T) {
		appCallUnaryServerError(t, app)
	})

	t.Run("appCallUnary", func(t *testing.T) {
		appCallUnary(t, app, buf)
	})

	t.Run("appCallStreamOutput", func(t *testing.T) {
		buf.Reset()
		appCallStreamOutput(t, app, buf)
	})

	t.Run("appCallStreamOutputError", func(t *testing.T) {
		appCallStreamOutputError(t, app)
	})
}

func appCallUnaryServerError(t *testing.T, app *app) {
	m, ok := findMethod(t, app, "grpc.testing.TestService", "UnaryCall")
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

	err := app.callUnary(context.Background(), m, []byte(msg))
	if err == nil {
		t.Error("error expected, got nil")
		return
	}

	s, _ := status.FromError(errors.Cause(err))
	if s.Code() != codes.Code(errCode) {
		t.Errorf("expectd status code %v, got %v", codes.Code(errCode), s.Code())
	}
}

func appCallUnary(t *testing.T, app *app, buf *bytes.Buffer) {
	m, ok := findMethod(t, app, "grpc.testing.TestService", "UnaryCall")
	if !ok {
		return
	}

	payloadType := "UNCOMPRESSABLE"
	body := base64.StdEncoding.EncodeToString([]byte("testBody"))

	msgTmpl := `
{
  "payload": {
    "type": "%s",
    "body": "%s"
  }
}
`

	msg := fmt.Sprintf(msgTmpl, payloadType, body)

	err := app.callUnary(context.Background(), m, []byte(msg))
	if err != nil {
		t.Errorf("error executing callUnary(): %v", err)
		return
	}

	res := buf.Bytes()
	root, err := ajson.Unmarshal(res)
	if err != nil {
		t.Errorf("error unmarshaling result json: %v", err)
		return
	}

	if jsonString(root, "$.payload.type") != payloadType {
		t.Errorf("payload type not found: %s", res)
		return
	}

	if jsonString(root, "$.payload.body") != body {
		t.Errorf("payload body not found: %s", res)
	}
}

func appCallStreamOutput(t *testing.T, app *app, buf *bytes.Buffer) {
	m, ok := findMethod(t, app, "grpc.testing.TestService", "StreamingOutputCall")
	if !ok {
		return
	}

	respSize1 := 3
	respSize2 := 5
	payloadType := "UNCOMPRESSABLE"
	bodyText := "testBody"

	msgTmpl := `
{
  "payload": {
    "type": "%s",
    "body": "%s"
  },
  "response_parameters": [{
    "size": %d
  },{
    "size": %d
  }]
}
`

	getEncBody := func(c int) string {
		body := strings.Repeat(bodyText, c)
		return base64.StdEncoding.EncodeToString([]byte(body))
	}

	msg := fmt.Sprintf(msgTmpl, payloadType, getEncBody(1), respSize1, respSize2)

	err := app.callServerStream(context.Background(), m, []byte(msg))
	if err != nil {
		t.Errorf("error executing callUnary(): %v", err)
		return
	}

	res := buf.Bytes()
	root, err := ajson.Unmarshal(res)
	if err != nil {
		t.Errorf("error unmarshaling result json: %v", err)
		return
	}

	if len(root.MustArray()) < 2 {
		t.Errorf("expected %d elements", 2)
		return
	}

	if jsonString(root, "$[0].payload.body") != getEncBody(respSize1) {
		t.Errorf("payload body[0] not found: %s", root)
		return
	}

	if jsonString(root, "$[1].payload.body") != getEncBody(respSize2) {
		t.Errorf("payload body[1] not found: %s", root)
	}
}

func appCallStreamOutputError(t *testing.T, app *app) {
	m, ok := findMethod(t, app, "grpc.testing.TestService", "StreamingOutputCall")
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

	err := app.callServerStream(context.Background(), m, []byte(msg))
	if err == nil {
		t.Error("error expected, got nil")
		return
	}

	s, _ := status.FromError(errors.Cause(err))
	if s.Code() != codes.Code(errCode) {
		t.Errorf("expectd status code %v, got %v", codes.Code(errCode), s.Code())
	}
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
