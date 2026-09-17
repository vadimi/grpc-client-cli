// Package app contains the interactive terminal application of
// grpc-client-cli built on top of bubbletea and bubbles: the service and
// method selection, the request editor, the call execution and the results
// display.
package app

import "errors"

var (
	// ErrInterrupted is returned when the user quits the interactive
	// application by pressing ctrl+c.
	ErrInterrupted = errors.New("prompt interrupted")
)
