package rpc

import "testing"

func TestConnectionOptionsParseHost(t *testing.T) {
	host := "test1.app"
	tests := []string{host, "host=" + host}

	for _, test := range tests {
		opts, _ := NewConnectionOpts(test)

		if opts.Host != host {
			t.Errorf("Expected %s, got %s", host, opts.Host)
		}
	}
}

func TestConnectionOptionsParseProxy(t *testing.T) {
	proxy := "testproxy.app"
	tests := []string{"host=test1.app,authority=" + proxy, "test1.app,authority=" + proxy}

	for _, test := range tests {
		opts, _ := NewConnectionOpts(test)

		if opts.Authority != proxy {
			t.Errorf("Expected %s, got %s", proxy, opts.Authority)
		}

		if opts.Host != "test1.app" {
			t.Errorf("Expected %s, got %s", "test1.app", opts.Host)
		}
	}
}

func TestConnectionOptionsParseError(t *testing.T) {
	_, err := NewConnectionOpts("")

	if err == nil {
		t.Error("Expected non nil error")
	}
}

func TestConnectionOptionsParseMetadata(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test,metadata=key1:", ""},
		{"test,metadata=key1:value1", "value1"},
		{"test,metadata=key1:value1:value2", "value1:value2"},
		{"test,metadata=key1:value1,metadata=key2:value2", "value1"},
		{"test,metadata=key1:value1,metadata=key1:value2", "value1"},
	}

	for _, test := range tests {

		opts, _ := NewConnectionOpts(test.input)

		val := opts.Metadata["key1"][0]
		if val != test.expected {
			t.Errorf("Metadata parse expected %s, got %s", test.expected, val)
		}
	}
}
