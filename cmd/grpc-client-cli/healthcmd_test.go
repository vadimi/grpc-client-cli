package main

import (
	"bytes"
	"errors"
	"flag"
	"testing"

	"github.com/spyzhov/ajson"
	"github.com/urfave/cli/v2"
	app_testing "github.com/vadimi/grpc-client-cli/internal/testing"
)

func TestHealthCheck(t *testing.T) {
	cases := []struct {
		name      string
		service   string
		expStatus string
		expErr    bool
	}{
		{name: "Healthy", service: "", expStatus: "SERVING"},
		{name: "Unhealthy", service: "unhealthy", expStatus: "NOT_SERVING", expErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			set := flag.NewFlagSet("test", 0)
			set.String("deadline", "15", "")
			set.String("service", c.service, "")
			set.Parse([]string{app_testing.TestServerAddr()})
			ctx := cli.NewContext(nil, set, nil)

			buf := &bytes.Buffer{}

			err := checkHealth(ctx, buf)
			if err != nil {
				if !c.expErr {
					t.Errorf("no error expected while checking health, got %v", err)
				}
			} else if c.expErr {
				t.Error("expected error, got nil")
			}

			res := buf.Bytes()
			root, err := ajson.Unmarshal(res)
			if err != nil {
				t.Errorf("error unmarshaling result json: %v", err)
				return
			}

			if jsonString(root, "$.status") != c.expStatus {
				t.Errorf("invalid health check status: %s, expected: %s", res, c.expStatus)
				return
			}
		})
	}
}

func TestHealthCheckError(t *testing.T) {
	cases := []struct {
		name    string
		service string
	}{
		{name: "Error", service: "error"},
		{name: "Unhealthy", service: "unhealthy"},
	}

	expectedExitCode := 1
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			set := flag.NewFlagSet("test", 0)
			set.Int("deadline", 15, "")
			set.String("service", c.service, "")
			set.Parse([]string{app_testing.TestServerAddr()})
			ctx := cli.NewContext(nil, set, nil)

			buf := &bytes.Buffer{}

			err := checkHealth(ctx, buf)
			if err == nil {
				t.Error("error expected")
			}

			var ec cli.ExitCoder
			errors.As(err, &ec)
			if ec.ExitCode() != expectedExitCode {
				t.Errorf("wrong exit code: %d, expected: %d", ec.ExitCode(), expectedExitCode)
			}
		})
	}
}
