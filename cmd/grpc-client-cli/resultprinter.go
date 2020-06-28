package main

import (
	"fmt"
	"io"
	"regexp"

	"github.com/vadimi/grpc-client-cli/internal/caller"
)

// collapse empty array to one line
var collapseArr = regexp.MustCompile(`\[\s*?\]`)

type resultPrinter interface {
	BeginArray()
	ArrayDelim()
	EndArray()
	WriteMessage([]byte)
}

func newResultPrinter(w io.Writer, f caller.MsgFormat) resultPrinter {
	if f == caller.Text {
		return &resultPrinterText{w}
	}

	return &resultPrinterJSON{w}
}

type resultPrinterJSON struct {
	w io.Writer
}

func (r *resultPrinterJSON) BeginArray() {
	fmt.Fprint(r.w, "[")
}

func (r *resultPrinterJSON) EndArray() {
	fmt.Fprintln(r.w, "]")
}

func (r *resultPrinterJSON) ArrayDelim() {
	fmt.Fprintln(r.w, ",")
}

func (r *resultPrinterJSON) WriteMessage(b []byte) {
	fmt.Fprintf(r.w, "%s", collapseArr.ReplaceAll(b, []byte("[]")))
}

type resultPrinterText struct {
	w io.Writer
}

func (r *resultPrinterText) BeginArray() {}

func (r *resultPrinterText) EndArray() {
	fmt.Fprintln(r.w)
}

func (r *resultPrinterText) ArrayDelim() {
	fmt.Fprint(r.w, "\n\n")
}

func (r *resultPrinterText) WriteMessage(b []byte) {
	fmt.Fprintf(r.w, "%s", b)
}
