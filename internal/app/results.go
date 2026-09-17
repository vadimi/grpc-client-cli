package app

import (
	"bytes"
	"fmt"
	"io"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/vadimi/grpc-client-cli/internal/caller"
)

// resultsPanel accumulates call results through a ResultPrinter and shows
// them in a scrollable viewport that follows newly arrived results.
type resultsPanel struct {
	// buf holds the plain results, out the same results colored for display
	buf     bytes.Buffer
	out     bytes.Buffer
	hl      *highlighter
	printer ResultPrinter
	stream  *StreamPrinter
	view    viewport.Model
}

func newResultsPanel(outFormat caller.MsgFormat) *resultsPanel {
	r := &resultsPanel{view: viewport.New()}
	if outFormat != caller.Text {
		hl := newHighlighter(true)
		r.hl = &hl
	}
	r.printer = NewResultPrinter(highlightWriter{plain: &r.buf, styled: &r.out, hl: r.hl}, outFormat)
	return r
}

// applyTheme switches the result colors to a light or dark palette. It
// affects results that arrive afterwards.
func (r *resultsPanel) applyTheme(t theme) {
	if r.hl != nil {
		*r.hl = newHighlighter(t.isDark)
	}
}

// Update forwards messages to the viewport for scrolling.
func (r *resultsPanel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	r.view, cmd = r.view.Update(msg)
	return cmd
}

// View renders the visible results.
func (r *resultsPanel) View() string {
	return r.view.View()
}

// SetSize resizes the results viewport.
func (r *resultsPanel) SetSize(width, height int) {
	r.view.SetWidth(width)
	r.view.SetHeight(height)
	r.refresh()
}

// reset clears all results for a new call.
func (r *resultsPanel) reset() {
	r.buf.Reset()
	r.out.Reset()
	r.stream = nil
	r.refresh()
}

// beginStream starts a streaming call result array.
func (r *resultsPanel) beginStream() {
	r.stream = NewStreamPrinter(r.printer)
	r.stream.Begin()
	r.refresh()
}

// addStreamResult appends a streaming call result.
func (r *resultsPanel) addStreamResult(b []byte) {
	if r.stream != nil {
		r.stream.Add(b)
		r.refresh()
	}
}

// endStream finishes a streaming call result array.
func (r *resultsPanel) endStream() {
	if r.stream != nil {
		r.stream.End()
		r.stream = nil
		r.refresh()
	}
}

// addSingle writes the result of a single-result call.
func (r *resultsPanel) addSingle(b []byte) {
	r.printer.WriteSingle(b)
	r.refresh()
}

// appendText appends a plain text line, e.g. a call error.
func (r *resultsPanel) appendText(s string) {
	fmt.Fprintln(&r.buf, s)
	fmt.Fprintln(&r.out, s)
	r.refresh()
}

// appendFn appends whatever fn writes, e.g. call statistics.
func (r *resultsPanel) appendFn(fn func(io.Writer)) {
	fn(highlightWriter{plain: &r.buf, styled: &r.out})
	r.refresh()
}

// refresh syncs the viewport with the accumulated results and follows
// them.
func (r *resultsPanel) refresh() {
	r.view.SetContent(r.out.String())
	r.view.GotoBottom()
}
