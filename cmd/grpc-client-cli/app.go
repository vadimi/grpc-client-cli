package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	grpcapp "github.com/vadimi/grpc-client-cli/internal/app"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type app struct {
	connFact     *rpc.GrpcConnFactory
	servicesList caller.ServiceMetaList
	exec         grpcapp.CallExecutor
	printer      grpcapp.ResultPrinter
	opts         *startOpts
	w            io.Writer
}

type startOpts struct {
	Service            string
	Method             string
	Discover           bool
	Deadline           int
	Verbose            bool
	Target             string
	IsInteractive      bool
	Authority          string
	InFormat           caller.MsgFormat
	OutFormat          caller.MsgFormat
	OutJsonNames       bool
	GrpcReflectVersion caller.GrpcReflectVersion

	// connection credentials
	TLS      bool
	Insecure bool
	CACert   string
	Cert     string
	CertKey  string

	Protos       []string
	ProtoImports []string
	Headers      map[string][]string

	Keepalive     bool
	KeepaliveTime time.Duration

	MaxRecvMsgSize int

	w io.Writer
}

func newApp(opts *startOpts) (*app, error) {
	connOpts := []rpc.ConnFactoryOption{
		rpc.WithAuthority(opts.Authority),
		rpc.WithKeepalive(opts.Keepalive, opts.KeepaliveTime),
	}

	if opts.TLS {
		connOpts = append(connOpts, rpc.WithConnCred(opts.Insecure, opts.CACert, opts.Cert, opts.CertKey))
	}

	if opts.MaxRecvMsgSize > 0 {
		connOpts = append(connOpts, rpc.WithMaxRecvMsgSize(opts.MaxRecvMsgSize))
	}

	if len(opts.Headers) > 0 {
		connOpts = append(connOpts, rpc.WithHeaders(opts.Headers))
	}

	a := &app{
		connFact: rpc.NewGrpcConnFactory(connOpts...),
		opts:     opts,
	}

	a.w = opts.w
	if a.w == nil {
		a.w = os.Stdout
	}

	a.exec = grpcapp.NewCallExecutor(a.connFact, opts.Target, opts.InFormat, opts.OutFormat, opts.OutJsonNames)
	a.printer = grpcapp.NewResultPrinter(a.w, opts.OutFormat)

	var svc caller.ServiceMetaData
	if len(opts.Protos) > 0 {
		svc = caller.NewServiceMetadataProto(opts.Protos, opts.ProtoImports)
	} else {
		svc = caller.NewServiceMetaData(&caller.ServiceMetaDataConfig{
			ConnFact:       a.connFact,
			Target:         a.opts.Target,
			Deadline:       a.opts.Deadline,
			ProtoImports:   a.opts.ProtoImports,
			ReflectVersion: a.opts.GrpcReflectVersion,
		})
	}

	ctx := rpc.WithStatsCtx(context.Background())
	services, err := svc.GetServiceMetaDataList(ctx)
	if err != nil {
		if a.opts.Verbose {
			grpcapp.PrintVerbose(a.w, rpc.ExtractRpcStats(ctx), err)
		}
		return nil, err
	}

	additionalFiles, err := svc.GetAdditionalFiles()
	if err != nil {
		return nil, err
	}

	err = caller.RegisterFiles(append(services.Files(), additionalFiles...)...)
	if err != nil && a.opts.Verbose {
		fmt.Println(err)
	}

	a.servicesList = services

	return a, nil
}

// Start runs the application: the interactive terminal ui when no message
// was provided, a single call otherwise.
func (a *app) Start(message []byte) error {
	if a.opts.IsInteractive {
		return a.startInteractive()
	}

	return a.startOnce(message)
}

// startInteractive runs the whole interactive session as a bubbletea
// application.
func (a *app) startInteractive() error {
	cfg := &grpcapp.Config{
		Services:  a.servicesList,
		Executor:  a.exec,
		Service:   a.opts.Service,
		Method:    a.opts.Method,
		Deadline:  a.opts.Deadline,
		Verbose:   a.opts.Verbose,
		InFormat:  a.opts.InFormat,
		OutFormat: a.opts.OutFormat,
		Discover:  a.opts.Discover,
	}

	res, err := grpcapp.Run(cfg)
	if err != nil {
		if errors.Is(err, grpcapp.ErrInterrupted) {
			// the user quit the session with ctrl+c
			return nil
		}
		return err
	}

	if a.opts.Discover {
		return a.printService(res.Service)
	}

	return nil
}

// startOnce resolves the requested service and method by name and performs
// a single call with the provided message.
func (a *app) startOnce(message []byte) error {
	service, err := grpcapp.ResolveService(a.servicesList, a.opts.Service)
	if err != nil {
		return err
	}

	if a.opts.Discover {
		return a.printService(service)
	}

	method, err := grpcapp.ResolveMethod(a.getService(service), a.opts.Method)
	if err != nil {
		return err
	}

	return a.callService(method, message)
}

func (a *app) Close() error {
	return a.connFact.Close()
}

// callService performs a single call with a message from a file or stdin.
func (a *app) callService(method protoreflect.MethodDescriptor, message []byte) error {
	var messages [][]byte
	var err error
	if method.IsStreamingClient() {
		if a.opts.InFormat == caller.JSON {
			messages, err = toJSONArray(message)
		} else {
			// TODO: parse text format array
			messages = append(messages, message)
		}
	} else {
		messages = append(messages, message)
	}

	if err != nil {
		return err
	}

	callTimeout := time.Duration(a.opts.Deadline) * time.Second
	ctx, cancel := context.WithTimeout(rpc.WithStatsCtx(context.Background()), callTimeout)
	defer cancel()

	if method.IsStreamingServer() {
		err = a.callStream(ctx, method, messages)
	} else {
		err = a.callClientStream(ctx, method, messages)
	}

	if err != nil {
		if !caller.IsErrTransient(err) {
			return err
		}
		fmt.Printf("Error: %s\n", err)
	}

	if a.opts.Verbose {
		grpcapp.PrintVerbose(a.w, rpc.ExtractRpcStats(ctx), errors.Unwrap(err))
	}

	// the message was provided, so the call is done
	return nil
}

// callClientStream calls unary or client stream method
func (a *app) callClientStream(ctx context.Context, method protoreflect.MethodDescriptor, messageJSON [][]byte) error {
	result, err := a.exec.CallUnary(ctx, method, messageJSON)
	if err != nil {
		return err
	}

	a.printer.WriteSingle(result)

	return nil
}

// callStream calls both server or bi-directional stream methods
func (a *app) callStream(ctx context.Context, method protoreflect.MethodDescriptor, messageJSON [][]byte) error {
	sp := grpcapp.NewStreamPrinter(a.printer)
	sp.Begin()
	err := a.exec.CallStreaming(ctx, method, messageJSON, sp.Add)
	sp.End()

	return err
}

func (a *app) printService(name string) error {
	normalizedName := strings.ToLower(name)
	for _, s := range a.servicesList {
		if normalizedName != "" && strings.Contains(strings.ToLower(s.Name), normalizedName) {
			return grpcapp.PrintFile(a.w, s.File)
		}
	}
	return fmt.Errorf("service %s not found, cannot print", name)
}

func (a *app) getService(serviceName string) *caller.ServiceMeta {
	for _, s := range a.servicesList {
		if s.Name == serviceName {
			return s
		}
	}

	return nil
}

func toJSONArray(msg []byte) ([][]byte, error) {
	var jsArr []json.RawMessage
	var err error
	nmsg := bytes.TrimSpace(msg)
	if nmsg[0] == byte('{') {
		var js json.RawMessage
		err = json.Unmarshal(nmsg, &js)
		if err == nil {
			jsArr = append(jsArr, js)
		}
	} else {
		err = json.Unmarshal(nmsg, &jsArr)
	}

	if err != nil {
		return nil, err
	}

	result := make([][]byte, len(jsArr))
	for i := range jsArr {
		result[i] = jsArr[i]
	}

	return result, nil
}
