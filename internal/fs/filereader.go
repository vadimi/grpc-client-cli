package fs

import (
	"io"
	"os"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

type noBomReader struct {
	io.Closer
	io.Reader
}

// NewFileReader creates io.ReadCloser that reads a file by ignoring BOM char
func NewFileReader(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return NewReader(f)
}

func NewReader(r io.ReadCloser) (io.ReadCloser, error) {
	bom := unicode.BOMOverride(unicode.UTF8.NewDecoder())
	unicodeReader := transform.NewReader(r, bom)
	br := &noBomReader{
		Closer: r,
		Reader: unicodeReader,
	}
	return br, nil
}
