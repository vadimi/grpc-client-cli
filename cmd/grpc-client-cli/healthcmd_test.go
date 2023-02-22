package main

import (
	"bytes"
	"errors"
	"flag"
	"testing"

	"github.com/spyzhov/ajson"
	"github.com/stretchr/testify/require"
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
			require.NoError(t, set.Parse([]string{app_testing.TestServerAddr()}))
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
			require.NoError(t, err, "error unmarshaling result json")
			require.Equal(t, c.expStatus, jsonString(root, "$.status"), "invalid heath check status")
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
			require.NoError(t, set.Parse([]string{app_testing.TestServerAddr()}))
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

func TestHealthCheckTLS(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.String("deadline", "15", "")
	set.String("service", "", "")
	set.String("tls", "true", "")
	set.String("cacert", "../../testdata/certs/test_ca.crt", "")
	require.NoError(t, set.Parse([]string{app_testing.TestServerTLSAddr()}))
	ctx := cli.NewContext(nil, set, nil)

	buf := &bytes.Buffer{}

	err := checkHealth(ctx, buf)
	require.NoError(t, err, "no error expected while checking health")

	res := buf.Bytes()
	root, err := ajson.Unmarshal(res)
	require.NoError(t, err, "error unmarshaling result json")
	require.Equal(t, "SERVING", jsonString(root, "$.status"), "invalid heath check status")
}
