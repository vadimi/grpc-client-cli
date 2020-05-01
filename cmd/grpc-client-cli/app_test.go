package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"

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

func TestAppCallUnaryServerError(t *testing.T) {
	app, err := newApp(&startOpts{
		Target:        app_testing.TestServerAddr(),
		Deadline:      15,
		IsInteractive: false,
	})

	var buf bytes.Buffer
	app.w = &buf

	if err != nil {
		t.Error(err)
		return
	}

	m, err := app.selectMethod(app.getService("grpc.testing.TestService"), "UnaryCall")
	if err != nil {
		t.Error(err)
		return
	}

	if m == nil {
		t.Error("method not found")
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

	err = app.callUnary(context.Background(), m, []byte(msg))
	if err == nil {
		t.Error("error expected, got nil")
	}

	s, _ := status.FromError(errors.Cause(err))
	if s.Code() != codes.Code(errCode) {
		t.Errorf("expectd status code %v, got %v", codes.Code(errCode), s.Code())
	}
}

func TestAppCallUnary(t *testing.T) {
	app, err := newApp(&startOpts{
		Target:        app_testing.TestServerAddr(),
		Deadline:      15,
		IsInteractive: false,
	})

	var buf bytes.Buffer
	app.w = &buf

	if err != nil {
		t.Error(err)
		return
	}

	m, err := app.selectMethod(app.getService("grpc.testing.TestService"), "UnaryCall")
	if err != nil {
		t.Error(err)
		return
	}

	if m == nil {
		t.Error("method not found")
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

	err = app.callUnary(context.Background(), m, []byte(msg))
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
	}

	if jsonString(root, "$.payload.body") != body {
		t.Errorf("payload body not found: %s", res)
	}
}

func TestAppCallStreamOutput(t *testing.T) {
	app, err := newApp(&startOpts{
		Target:        app_testing.TestServerAddr(),
		Deadline:      15,
		IsInteractive: false,
	})

	var buf bytes.Buffer
	app.w = &buf

	if err != nil {
		t.Error(err)
		return
	}

	m, err := app.selectMethod(app.getService("grpc.testing.TestService"), "StreamingOutputCall")
	if err != nil {
		t.Error(err)
		return
	}

	if m == nil {
		t.Error("method not found")
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

	err = app.callServerStream(context.Background(), m, []byte(msg))
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
		t.Errorf("payload body not found: %s", root)
	}

	if jsonString(root, "$[1].payload.body") != getEncBody(respSize2) {
		t.Errorf("payload body not found: %s", root)
	}
}

func jsonString(n *ajson.Node, jsonPath string) string {
	nodes, err := n.JSONPath(jsonPath)
	if err != nil {
		panic(err)
	}

	return nodes[0].MustString()
}
