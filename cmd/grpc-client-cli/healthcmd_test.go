package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/spyzhov/ajson"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
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
			cmd := &cli.Command{
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "deadline", Value: "15"},
					&cli.StringFlag{Name: "service"},
					&cli.StringFlag{Name: "address"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return nil
				},
			}
			require.NoError(t, cmd.Run(context.Background(), []string{"test", "--service", c.service, "--address", app_testing.TestServerAddr()}))

			buf := &bytes.Buffer{}

			err := checkHealth(context.Background(), cmd, buf)
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
			cmd := &cli.Command{
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "deadline", Value: "15"},
					&cli.StringFlag{Name: "service"},
					&cli.StringFlag{Name: "address"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return nil
				},
			}
			require.NoError(t, cmd.Run(context.Background(), []string{"test", "--service", c.service, "--address", app_testing.TestServerAddr()}))

			buf := &bytes.Buffer{}

			err := checkHealth(context.Background(), cmd, buf)
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
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "deadline", Value: "15s"},
			&cli.StringFlag{Name: "service"},
			&cli.StringFlag{Name: "address"},
			&cli.BoolFlag{Name: "tls"},
			&cli.StringFlag{Name: "cacert"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return nil
		},
	}
	require.NoError(t, cmd.Run(context.Background(), []string{"test", "--service", "", "--address", app_testing.TestServerTLSAddr(), "--tls", "--cacert", "../../testdata/certs/test_ca.crt"}))

	buf := &bytes.Buffer{}

	err := checkHealth(context.Background(), cmd, buf)
	require.NoError(t, err, "no error expected while checking health")

	res := buf.Bytes()
	root, err := ajson.Unmarshal(res)
	require.NoError(t, err, "error unmarshaling result json")
	require.Equal(t, "SERVING", jsonString(root, "$.status"), "invalid heath check status")
}
