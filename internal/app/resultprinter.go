package app

import (
	"fmt"
	"io"
	"regexp"

	"github.com/vadimi/grpc-client-cli/internal/caller"
)

// collapseArr collapses empty arrays to one line.
var collapseArr = regexp.MustCompile(`\[\s*?\]`)

// ResultPrinter formats call results for an output stream.
type ResultPrinter interface {
	// WriteSingle writes the result of a single-result call.
	WriteSingle(b []byte)
	// BeginArray starts a result array of a streaming call.
	BeginArray()
	// ArrayDelim writes the delimiter between streaming results.
	ArrayDelim()
	// EndArray finishes a result array of a streaming call.
	EndArray()
	// WriteMessage writes a single streaming result.
	WriteMessage(b []byte)
}

// NewResultPrinter returns a printer for the given output format.
func NewResultPrinter(w io.Writer, f caller.MsgFormat) ResultPrinter {
	if f == caller.Text {
		return &resultPrinterText{w: w}
	}

	return &resultPrinterJSON{w: w}
}

type resultPrinterJSON struct {
	w io.Writer
}

func (r *resultPrinterJSON) WriteSingle(b []byte) {
	r.WriteMessage(b)
	fmt.Fprintln(r.w)
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
	//nolint:errcheck // writing results must not abort a call
	fmt.Fprintf(r.w, "%s", collapseArr.ReplaceAll(b, []byte("[]")))
}

type resultPrinterText struct {
	w io.Writer
}

func (r *resultPrinterText) WriteSingle(b []byte) {
	r.WriteMessage(b)
	fmt.Fprintln(r.w)
}

func (r *resultPrinterText) BeginArray() {}

func (r *resultPrinterText) EndArray() {
	fmt.Fprintln(r.w)
}

func (r *resultPrinterText) ArrayDelim() {
	fmt.Fprint(r.w, "\n\n")
}

func (r *resultPrinterText) WriteMessage(b []byte) {
	//nolint:errcheck // writing results must not abort a call
	fmt.Fprintf(r.w, "%s", b)
}

// StreamPrinter drives a ResultPrinter through the sequence of a streaming
// call: one BeginArray, results separated by ArrayDelim and one EndArray,
// even when the call produced no results.
type StreamPrinter struct {
	p    ResultPrinter
	next bool
}

// NewStreamPrinter returns a stream printer writing to p.
func NewStreamPrinter(p ResultPrinter) *StreamPrinter {
	return &StreamPrinter{p: p}
}

// Begin starts the result array.
func (s *StreamPrinter) Begin() {
	s.p.BeginArray()
}

// Add appends a streaming result.
func (s *StreamPrinter) Add(b []byte) {
	if s.next {
		s.p.ArrayDelim()
	}
	s.p.WriteMessage(b)
	s.next = true
}

// End finishes the result array.
func (s *StreamPrinter) End() {
	s.p.EndArray()
}
