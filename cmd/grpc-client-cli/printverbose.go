package main

import (
	"fmt"
	"io"

	"github.com/pterm/pterm"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	"google.golang.org/grpc/status"
)

func printVerbose(w io.Writer, s *rpc.Stats, rpcErr error) {
	fmt.Fprintln(w)

	pterm.Fprintln(w, pterm.Bold.Sprint("Method: ")+s.FullMethod())

	rpcStatus := status.Code(rpcErr)
	pterm.Fprintln(w, pterm.Bold.Sprint("Status: ")+pterm.FgLightYellow.Sprintf("%d", rpcStatus)+" "+pterm.Italic.Sprint(rpcStatus))

	pterm.Fprintln(w, pterm.Italic.Sprint("\nRequest Headers:"))
	for k, v := range s.ReqHeaders() {
		pterm.Fprintln(w, pterm.Bold.Sprint(k+": ")+pterm.LightGreen(v))
	}

	if s.RespHeaders().Len() > 0 {
		pterm.Fprintln(w, pterm.Italic.Sprint("\nResponse Headers:"))
		for k, v := range s.RespHeaders() {
			pterm.Fprintln(w, pterm.Bold.Sprint(k+": ")+pterm.LightGreen(v))
		}
	}

	if s.RespTrailers().Len() > 0 {
		pterm.Fprintln(w, pterm.Italic.Sprint("\nResponse Trailers:"))
		for k, v := range s.RespTrailers() {
			pterm.Fprintln(w, pterm.Bold.Sprint(k+": ")+pterm.LightGreen(v))
		}
	}

	fmt.Fprintln(w)
	pterm.Fprintln(w, pterm.Bold.Sprint("Request duration: ")+pterm.FgLightYellow.Sprint(s.Duration))
	pterm.Fprintln(w, pterm.Bold.Sprint("Request size: ")+pterm.FgLightYellow.Sprintf("%d bytes", s.ReqSize()))
	pterm.Fprintln(w, pterm.Bold.Sprint("Response size: ")+pterm.FgLightYellow.Sprintf("%d bytes", s.RespSize()))
	fmt.Fprintln(w)
}
