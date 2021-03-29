package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/stretchr/testify/require"
	app_testing "github.com/vadimi/grpc-client-cli/internal/testing"
)

func TestDiscoverCommand(t *testing.T) {
	app, err := newApp(&startOpts{
		Target:        app_testing.TestServerAddr(),
		Deadline:      15,
		IsInteractive: false,
		Discover:      true,
		Service:       "TestService",
	})

	buf := &bytes.Buffer{}
	app.w = buf

	if err != nil {
		t.Error(err)
		return
	}

	err = app.Start([]byte("{}"))
	if err != nil {
		t.Error(err)
		return
	}

	res := buf.String()

	if !strings.Contains(res, "service TestService") {
		t.Errorf("expected TestService service def, got %s", res)
		return
	}

	services, err := grpcreflect.LoadServiceDescriptors(app_testing.TestServerInstance())
	if err != nil {
		t.Error(err)
		return
	}

	testSvc, ok := services["grpc_client_cli.testing.TestService"]
	require.True(t, ok, "grpc service not found")
	for _, m := range testSvc.GetMethods() {
		if !strings.Contains(res, "rpc "+m.GetName()) {
			t.Errorf("expected %s method def, got %s", m.GetName(), res)
			return
		}
	}

	for _, msg := range testSvc.GetFile().GetMessageTypes() {
		if !strings.Contains(res, "message "+msg.GetName()) {
			t.Errorf("expected %s message def, got %s", msg.GetName(), res)
			return
		}
	}
}
