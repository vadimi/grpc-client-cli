package main

import (
	"fmt"
	"io"

	"github.com/gookit/color"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoprint"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func printFile(w io.Writer, f protoreflect.FileDescriptor) error {
	wrappedFile, err := desc.WrapFile(f)
	if err != nil {
		return err
	}
	p := &protoprint.Printer{}
	return p.PrintProtoFile(wrappedFile, w)
}

func printVerbose(w io.Writer, s *rpc.Stats, rpcErr error) {
	fmt.Fprintln(w)

	fmt.Fprintln(w, color.Bold.Sprint("Method: ")+s.FullMethod())

	rpcStatus := status.Code(rpcErr)
	fmt.Fprintln(w, color.Bold.Sprint("Status: ")+color.FgLightYellow.Sprintf("%d", rpcStatus)+" "+color.OpItalic.Sprint(rpcStatus))

	fmt.Fprintln(w, color.OpItalic.Sprint("\nRequest Headers:"))
	for k, v := range s.ReqHeaders() {
		fmt.Fprintln(w, color.Bold.Sprint(k+": ")+color.LightGreen.Sprint(v))
	}

	if s.RespHeaders().Len() > 0 {
		fmt.Fprintln(w, color.OpItalic.Sprint("\nResponse Headers:"))
		for k, v := range s.RespHeaders() {
			fmt.Fprintln(w, color.Bold.Sprint(k+": ")+color.LightGreen.Sprint(v))
		}
	}

	if s.RespTrailers().Len() > 0 {
		color.Fprintln(w, color.OpItalic.Sprint("\nResponse Trailers:"))
		for k, v := range s.RespTrailers() {
			fmt.Fprintln(w, color.Bold.Sprint(k+": ")+color.LightGreen.Sprint(v))
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, color.Bold.Sprint("Request duration: ")+color.FgLightYellow.Sprint(s.Duration))
	fmt.Fprintln(w, color.Bold.Sprint("Request size: ")+color.FgLightYellow.Sprintf("%d bytes", s.ReqSize()))
	fmt.Fprintln(w, color.Bold.Sprint("Response size: ")+color.FgLightYellow.Sprintf("%d bytes", s.RespSize()))
	fmt.Fprintln(w)
}
