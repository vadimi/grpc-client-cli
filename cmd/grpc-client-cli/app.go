package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/vadimi/grpc-client-cli/internal/services"

	"github.com/golang/protobuf/jsonpb"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoprint"
	"github.com/jhump/protoreflect/dynamic"
	"google.golang.org/grpc"
	survey "gopkg.in/AlecAivazis/survey.v1"
	"gopkg.in/AlecAivazis/survey.v1/core"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
)

var (
	NoMethodErr = errors.New("no method")
)

type app struct {
	connFact      *services.GrpcConnFactory
	servicesList  []*services.ServiceMeta
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
		connFact: services.NewGrpcConnFactory(),
		opts:     opts,
	}

	if a.w == nil {
		a.w = os.Stdout
	}

	svc := services.NewServiceMetaData(a.connFact)
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

		err = a.callService(method, message, a.opts.Deadline)
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

func (a *app) callService(method *desc.MethodDescriptor, message []byte, deadline int) error {
	for {
		var err error
		selectedMsg := message
		if len(selectedMsg) == 0 {
			selectedMsg, err = a.selectMessage(method.GetInputType())
			if err != nil {
				return err
			}
		}

		ctx, cancel := context.WithTimeout(services.DiagContext(), time.Duration(deadline)*time.Second)
		if method.IsServerStreaming() && !method.IsClientStreaming() {
			err = a.callServerStream(ctx, method, selectedMsg)
		} else if !method.IsServerStreaming() && !method.IsClientStreaming() {
			err = a.callUnary(ctx, method, selectedMsg)
		} else {
			err = errors.New("client/bi-directional streaming is not supported")
		}

		if err != nil {
			if !services.IsTemporary(err) {
				cancel()
				return err
			}
			fmt.Printf("Error: %s\n", err)
		}

		if a.opts.Verbose {
			di := services.ExtractDiagInfo(ctx)
			fmt.Println()
			fmt.Println("Request duration:", di.Duration)
			fmt.Printf("Request size: %d bytes\n", di.ReqSize)
			fmt.Printf("Response size: %d bytes\n", di.RespSize)
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
	serviceCaller := services.NewServiceCaller(a.connFact)

	result, err := serviceCaller.CallJSON(ctx, a.opts.Target, method, messageJSON, grpc.WaitForReady(true))
	if err != nil {
		return err
	}

	re := regexp.MustCompile(`\[\s*?\]`) // collapse empty array to one line
	a.w.Write(re.ReplaceAll(result, []byte("[]")))

	return nil
}

func (a *app) callServerStream(ctx context.Context, method *desc.MethodDescriptor, messageJSON []byte) error {
	serviceCaller := services.NewServiceCaller(a.connFact)
	result, errChan := serviceCaller.CallServerStream(ctx, a.opts.Target, method, messageJSON, grpc.WaitForReady(true))

	a.w.Write([]byte("["))
	cnt := 0
	for {
		select {
		case r := <-result:
			if r != nil {
				if cnt > 0 {
					a.w.Write([]byte(","))
				}
				a.w.Write(r)
				cnt++
			}
		case err := <-errChan:
			a.w.Write([]byte("]"))
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

func (a *app) selectMethod(s *services.ServiceMeta, name string) (*desc.MethodDescriptor, error) {
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

func (a *app) selectMessage(messageDesc *desc.MessageDescriptor) ([]byte, error) {
	fieldNames := a.getFieldNames(messageDesc)
	for {
		message, err := a.messageReader.ReadLine(fieldNames)
		if err != nil {
			if err == terminal.InterruptErr {
				return nil, terminal.InterruptErr
			}
			return message, err
		}

		normMsg := bytes.TrimSpace(message)
		if len(normMsg) > 0 {
			if bytes.Equal(normMsg, []byte("?")) {
				msg := dynamic.NewMessage(messageDesc)
				msgJSON, _ := msg.MarshalJSONPB(&jsonpb.Marshaler{
					EmitDefaults: true,
					OrigName:     true,
				})
				fmt.Println(string(msgJSON))
				continue
			}
			return normMsg, nil
		}
	}
}

func (a *app) getService(serviceName string) *services.ServiceMeta {
	for _, s := range a.servicesList {
		if s.Name == serviceName {
			return s
		}
	}

	return nil
}

func (a *app) getFieldNames(messageDesc *desc.MessageDescriptor) []string {
	fields := map[string]struct{}{}

	walker := services.NewFieldWalker()
	walker.Walk(messageDesc, func(f *desc.FieldDescriptor) {
		fields[f.GetName()] = struct{}{}
	})

	names := make([]string, 0, len(fields))
	for f := range fields {
		names = append(names, f)
	}

	sort.Strings(names)
	return names
}
