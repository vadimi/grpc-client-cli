package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
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
}

type startOpts struct {
	Service       string
	Method        string
	Discover      bool
	Deadline      int
	Verbose       bool
	Target        string
	IsInteractive bool
}

func newApp(opts *startOpts) (*app, error) {
	core.SelectFocusIcon = "→"

	a := &app{
		connFact: rpc.NewGrpcConnFactory(),
		opts:     opts,
	}

	if a.w == nil {
		a.w = os.Stdout
	}

	svc := caller.NewServiceMetaData(a.connFact)
	services, err := svc.GetServiceMetaDataList(a.opts.Target, a.opts.Deadline)
	if err != nil {
		return nil, err
	}

	a.servicesList = services

	rl, err := newMsgReader(&msgReaderSettings{
		Prompt:      "Message json (type ? to see defaults): ",
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
		var err error
		var messages [][]byte
		buf := newMsgBuffer(&msgBufferOptions{
			reader:      a.messageReader,
			messageDesc: method.GetInputType(),
		})

		selectedMsg := message
		if len(selectedMsg) == 0 {
			if method.IsClientStreaming() {
				messages, err = buf.ReadMessages()
			} else {
				selectedMsg, err = buf.ReadMessage()
			}
		} else if method.IsClientStreaming() {
			messages, err = toJSONArray(message)
		}

		if err != nil {
			return err
		}

		callTimeout := time.Duration(a.opts.Deadline) * time.Second
		ctx, cancel := context.WithTimeout(rpc.WithStatsCtx(context.Background()), callTimeout)
		if method.IsServerStreaming() && !method.IsClientStreaming() {
			err = a.callServerStream(ctx, method, selectedMsg)
		} else if !method.IsServerStreaming() && !method.IsClientStreaming() {
			err = a.callUnary(ctx, method, selectedMsg)
		} else if !method.IsServerStreaming() && method.IsClientStreaming() {
			err = a.callClientStream(ctx, method, messages)
		} else {
			err = errors.New("bi-directional streaming is not supported")
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

func (a *app) callUnary(ctx context.Context, method *desc.MethodDescriptor, messageJSON []byte) error {
	serviceCaller := caller.NewServiceCaller(a.connFact)

	result, err := serviceCaller.CallJSON(ctx, a.opts.Target, method, messageJSON, grpc.WaitForReady(true))
	if err != nil {
		return err
	}

	a.printResult(result)

	return nil
}

func (a *app) callClientStream(ctx context.Context, method *desc.MethodDescriptor, messageJSON [][]byte) error {
	serviceCaller := caller.NewServiceCaller(a.connFact)

	result, err := serviceCaller.CallClientStream(ctx, a.opts.Target, method, messageJSON, grpc.WaitForReady(true))
	if err != nil {
		return err
	}

	a.printResult(result)

	return nil
}

func (a *app) printResult(r []byte) {
	re := regexp.MustCompile(`\[\s*?\]`) // collapse empty array to one line
	fmt.Fprintf(a.w, "%s\n", re.ReplaceAll(r, []byte("[]")))
}

func (a *app) callServerStream(ctx context.Context, method *desc.MethodDescriptor, messageJSON []byte) error {
	serviceCaller := caller.NewServiceCaller(a.connFact)
	result, errChan := serviceCaller.CallServerStream(ctx, a.opts.Target, method, messageJSON, grpc.WaitForReady(true))

	fmt.Fprint(a.w, "[")
	cnt := 0
	for {
		select {
		case r := <-result:
			if r != nil {
				if cnt > 0 {
					fmt.Fprintln(a.w, ",")
				}
				a.w.Write(r)
				cnt++
			}
		case err := <-errChan:
			fmt.Fprintln(a.w, "]")
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
	fmt.Fprintf(a.w, "Request size: %d bytes\n", s.ReqSize)
	fmt.Fprintf(a.w, "Response size: %d bytes\n", s.RespSize)
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
