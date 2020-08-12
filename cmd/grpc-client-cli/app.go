package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoprint"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	"google.golang.org/grpc"
	survey "gopkg.in/AlecAivazis/survey.v1"
	"gopkg.in/AlecAivazis/survey.v1/core"
)

var (
	NoMethodErr = errors.New("no method")
)

type app struct {
	connFact      *rpc.GrpcConnFactory
	servicesList  []*caller.ServiceMeta
	messageReader *msgReader
	opts          *startOpts
	w             io.Writer
	printer       resultPrinter
}

type startOpts struct {
	Service       string
	Method        string
	Discover      bool
	Deadline      int
	Verbose       bool
	Target        string
	IsInteractive bool
	Authority     string
	Format        caller.MsgFormat

	// connection credentials
	TLS      bool
	Insecure bool
	CACert   string
	Cert     string
	CertKey  string

	Protos       []string
	ProtoImports []string

	w io.Writer
}

func newApp(opts *startOpts) (*app, error) {
	core.SelectFocusIcon = "→"

	connOpts := []rpc.ConnFactoryOption{rpc.WithAuthority(opts.Authority)}
	if opts.TLS {
		connOpts = append(connOpts, rpc.WithConnCred(opts.Insecure, opts.CACert, opts.Cert, opts.CertKey))
	}

	a := &app{
		connFact: rpc.NewGrpcConnFactory(connOpts...),
		opts:     opts,
	}

	a.w = opts.w
	if a.w == nil {
		a.w = os.Stdout
	}

	a.printer = newResultPrinter(a.w, opts.Format)

	var svc caller.ServiceMetaData
	if len(opts.Protos) > 0 {
		svc = caller.NewServiceMetadataProto(opts.Protos, opts.ProtoImports)
	} else {
		svc = caller.NewServiceMetaData(a.connFact, a.opts.Target, a.opts.Deadline)
	}
	services, err := svc.GetServiceMetaDataList()
	if err != nil {
		return nil, err
	}

	a.servicesList = services

	rl, err := newMsgReader(&msgReaderSettings{
		Prompt:      fmt.Sprintf("Message %s (type ? to see defaults): ", a.opts.Format.String()),
		HistoryFile: os.TempDir() + "/grpc-client-cli.tmp",
	})

	if err != nil {
		return nil, err
	}

	a.messageReader = rl

	return a, nil
}

func (a *app) Start(message []byte) error {
	for {
		service, err := a.selectService(a.opts.Service)
		if err != nil {
			return err
		}

		if a.opts.Discover {
			return a.printService(service)
		}

		method, err := a.selectMethod(a.getService(service), a.opts.Method)
		if err != nil {
			if err == NoMethodErr {
				continue
			}
			return err
		}

		err = a.callService(method, message)
		// Ctrl+D will trigger io.EOF if the line is empty
		// restart the app from the beginning
		if err != io.EOF {
			return err
		}
	}
}

func (a *app) Close() error {
	cerr := a.connFact.Close()
	if a.messageReader == nil {
		return cerr
	}

	if merr := a.messageReader.Close(); merr != nil {
		if cerr == nil {
			return merr
		}

		return errors.New(cerr.Error() + "; " + merr.Error())
	}

	return nil
}

func (a *app) callService(method *desc.MethodDescriptor, message []byte) error {
	for {
		buf := newMsgBuffer(&msgBufferOptions{
			reader:      a.messageReader,
			messageDesc: method.GetInputType(),
			msgFormat:   a.opts.Format,
		})

		var err error
		var messages [][]byte
		if len(message) == 0 {
			if method.IsClientStreaming() {
				messages, err = buf.ReadMessages()
			} else {
				var m []byte
				m, err = buf.ReadMessage()
				messages = append(messages, m)
			}
		} else {
			if method.IsClientStreaming() {
				if a.opts.Format == caller.JSON {
					messages, err = toJSONArray(message)
				} else {
					// TODO: parse text format array
					messages = append(messages, message)
				}
			} else {
				messages = append(messages, message)
			}
		}

		if err != nil {
			return err
		}

		callTimeout := time.Duration(a.opts.Deadline) * time.Second
		ctx, cancel := context.WithTimeout(rpc.WithStatsCtx(context.Background()), callTimeout)
		if method.IsServerStreaming() {
			err = a.callStream(ctx, method, messages)
		} else {
			err = a.callClientStream(ctx, method, messages)
		}

		if err != nil {
			if !caller.IsErrTransient(err) {
				cancel()
				return err
			}
			fmt.Printf("Error: %s\n", err)
		}

		if a.opts.Verbose {
			a.printVerboseOutput(ctx)
		}

		// if we pass a single message, return
		if len(message) > 0 {
			cancel()
			return nil
		}
		cancel()
	}
}

// callClientStream calls unary or client stream method
func (a *app) callClientStream(ctx context.Context, method *desc.MethodDescriptor, messageJSON [][]byte) error {
	serviceCaller := caller.NewServiceCaller(a.connFact, a.opts.Format)

	result, err := serviceCaller.CallClientStream(ctx, a.opts.Target, method, messageJSON, grpc.WaitForReady(true))
	if err != nil {
		return err
	}

	a.printResult(result)

	return nil
}

func (a *app) printResult(r []byte) {
	a.printer.WriteMessage(r)
	fmt.Fprintln(a.w)
}

// callStream calls both server or bi-directional stream methods
func (a *app) callStream(ctx context.Context, method *desc.MethodDescriptor, messageJSON [][]byte) error {
	serviceCaller := caller.NewServiceCaller(a.connFact, a.opts.Format)
	result, errChan := serviceCaller.CallStream(ctx, a.opts.Target, method, messageJSON, grpc.WaitForReady(true))

	a.printer.BeginArray()
	cnt := 0
	for {
		select {
		case r := <-result:
			if r != nil {
				if cnt > 0 {
					a.printer.ArrayDelim()
				}
				a.printer.WriteMessage(r)
				cnt++
			}
		case err := <-errChan:
			a.printer.EndArray()
			return err
		}
	}
}

func (a *app) selectService(name string) (string, error) {
	serviceNames := []string{}
	normalizedName := strings.ToLower(name)
	for _, s := range a.servicesList {
		if normalizedName != "" && strings.Contains(strings.ToLower(s.Name), normalizedName) {
			return s.Name, nil
		}
		serviceNames = append(serviceNames, s.Name)
	}

	if !a.opts.IsInteractive {
		return "", errors.New("service name not found or invalid")
	}

	// ascending sort for service names
	sort.Slice(serviceNames, func(i, j int) bool { return strings.ToLower(serviceNames[i]) < strings.ToLower(serviceNames[j]) })
	service := ""
	err := survey.AskOne(&survey.Select{
		Message:  "Choose a service:",
		Options:  serviceNames,
		PageSize: 20,
	}, &service, survey.Required)
	return service, err
}

func (a *app) printService(name string) error {
	normalizedName := strings.ToLower(name)
	for _, s := range a.servicesList {
		if normalizedName != "" && strings.Contains(strings.ToLower(s.Name), normalizedName) {
			p := &protoprint.Printer{}
			return p.PrintProtoFile(s.File, a.w)
		}
	}
	return fmt.Errorf("service %s not found, cannot print", name)
}

func (a *app) selectMethod(s *caller.ServiceMeta, name string) (*desc.MethodDescriptor, error) {
	noMethod := "[..]"
	methodNames := []string{noMethod}
	for _, m := range s.Methods {
		mn := m.GetName()
		if name != "" && strings.EqualFold(mn, name) {
			return m, nil
		}
		methodNames = append(methodNames, m.GetName())
	}

	if !a.opts.IsInteractive {
		return nil, errors.New("method name not found or invalid")
	}

	// ascending sort for method names
	sort.Slice(methodNames, func(i, j int) bool { return strings.ToLower(methodNames[i]) < strings.ToLower(methodNames[j]) })
	methodName := ""
	err := survey.AskOne(&survey.Select{
		Message:  "Choose a method:",
		Options:  methodNames,
		PageSize: 20,
	}, &methodName, survey.Required)
	if err != nil {
		return nil, err
	}

	if methodName == noMethod {
		return nil, NoMethodErr
	}

	for _, m := range s.Methods {
		if m.GetName() == methodName {
			return m, nil
		}
	}

	return nil, errors.New("method not found")
}

func (a *app) getService(serviceName string) *caller.ServiceMeta {
	for _, s := range a.servicesList {
		if s.Name == serviceName {
			return s
		}
	}

	return nil
}

func (a *app) printVerboseOutput(ctx context.Context) {
	s := rpc.ExtractRpcStats(ctx)
	fmt.Fprintln(a.w)
	fmt.Fprintln(a.w, "Request duration:", s.Duration)
	fmt.Fprintf(a.w, "Request size: %d bytes\n", s.ReqSize())
	fmt.Fprintf(a.w, "Response size: %d bytes\n", s.RespSize())
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
