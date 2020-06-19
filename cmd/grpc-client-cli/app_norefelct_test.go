package main

import (
	"testing"

	app_testing "github.com/vadimi/grpc-client-cli/internal/testing"
)

func TestAppServiceCallsNoReflect(t *testing.T) {
	runAppServiceCalls(t, &startOpts{
		Target:        app_testing.TestServerNoReflectAddr(),
		Deadline:      15,
		IsInteractive: false,
		Protos:        []string{"../../testdata/test.proto"},
	})
}
