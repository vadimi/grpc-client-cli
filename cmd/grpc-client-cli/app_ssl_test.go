package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	app_testing "github.com/vadimi/grpc-client-cli/internal/testing"
)

func TestAppServiceTLSInvalidCerts(t *testing.T) {
	_, err := newApp(&startOpts{
		Target:        app_testing.TestServerTLSAddr(),
		Deadline:      15,
		IsInteractive: false,
		TLS:           true,
		CACert:        "../../testdata/certs/other_ca.crt",
	})

	if err == nil {
		t.Errorf("certificate signature validation error is expected")
	}
}

func TestAppServiceTLSCalls(t *testing.T) {
	buf := &bytes.Buffer{}
	app, err := newApp(&startOpts{
		Target:        app_testing.TestServerTLSAddr(),
		Deadline:      15,
		IsInteractive: false,
		TLS:           true,
		CACert:        "../../testdata/certs/test_ca.crt",
		w:             buf,
	})
	if err != nil {
		t.Error(err)
		return
	}

	t.Run("appCallUnaryTLS", func(t *testing.T) {
		appCallUnary(t, app, buf)
	})
}

func TestAppServiceMTLSCalls(t *testing.T) {
	buf := &bytes.Buffer{}
	app, err := newApp(&startOpts{
		Target:        app_testing.TestServerMTLSAddr(),
		Deadline:      15,
		IsInteractive: false,
		TLS:           true,
		CACert:        "../../testdata/certs/test_ca.crt",
		Cert:          "../../testdata/certs/test_client.crt",
		CertKey:       "../../testdata/certs/test_client.key",
		w:             buf,
	})
	if err != nil {
		t.Error(err)
		return
	}

	t.Run("appCallUnaryTLS", func(t *testing.T) {
		appCallUnary(t, app, buf)
	})
}

func TestAppServiceMTLSInvalidCerts(t *testing.T) {
	tests := []struct {
		name    string
		cacert  string
		cert    string
		certkey string
	}{
		{
			name:    "Invalid CA",
			cacert:  "../../testdata/certs/other_ca.crt",
			cert:    "../../testdata/certs/other_client.crt",
			certkey: "../../testdata/certs/other_client.key",
		},
		{
			name:   "NoClientCerts",
			cacert: "../../testdata/certs/test_ca.crt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newApp(&startOpts{
				Target:        app_testing.TestServerMTLSAddr(),
				Deadline:      15,
				IsInteractive: false,
				TLS:           true,
				CACert:        tt.cacert,
				Cert:          tt.cert,
				CertKey:       tt.certkey,
			})

			if err == nil {
				t.Errorf("certificate signature validation error is expected")
			}
		})
	}
}

func TestAppServiceTLSInvalidCertsInsecure(t *testing.T) {
	_, err := newApp(&startOpts{
		Target:        app_testing.TestServerTLSAddr(),
		Deadline:      15,
		IsInteractive: false,
		Insecure:      true,
		TLS:           true,
		CACert:        "../../testdata/certs/other_ca.crt",
	})

	assert.NoError(t, err)
}
