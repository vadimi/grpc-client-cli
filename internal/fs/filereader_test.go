package fs

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestNoBomReaderWithBom(t *testing.T) {
	withBom := []byte{
		0xff, // BOM
		0xfe, // BOM
		'T',
		0x00,
		'E',
		0x00,
		'S',
		0x00,
		'T',
		0x00,
	}

	tests := []struct {
		name string
		src  []byte
	}{
		{name: "WithBom", src: withBom},
		{name: "WithoutBom", src: []byte("TEST")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := bytes.NewReader(test.src)

			br, _ := NewReader(ioutil.NopCloser(r))
			result, _ := ioutil.ReadAll(br)

			if string(result) != "TEST" {
				t.Errorf("expected TEST, got %s", result)
			}
		})
	}
}
