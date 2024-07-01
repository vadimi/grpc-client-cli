package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoprint"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/dynamicpb"
)

type msgBuffer struct {
	opts       *msgBufferOptions
	fieldNames []string
	// next message prompt
	nextPrompt string
	helpText   string
	protoText  string
	w          io.Writer
}

type msgBufferOptions struct {
	reader      MsgReader
	messageDesc *desc.MessageDescriptor
	msgFormat   caller.MsgFormat
	w           io.Writer
}

func newMsgBuffer(opts *msgBufferOptions) *msgBuffer {
	w := opts.w
	if w == nil {
		w = os.Stdout
	}
	return &msgBuffer{
		nextPrompt: "Next message (press Ctrl-D to finish): ",
		opts:       opts,
		fieldNames: fieldNames(opts.messageDesc),
		helpText:   getMessageDefaults(opts.messageDesc),
		protoText:  protoString(opts.messageDesc),
		w:          w,
	}
}

func (b *msgBuffer) ReadMessage(opts ...ReadLineOpt) ([]byte, error) {
	for {
		message, err := b.opts.reader.ReadLine(b.fieldNames, opts...)
		if err != nil {
			if err == ErrInterruptTerm {
				return nil, ErrInterruptTerm
			}
			return message, err
		}

		normMsg := bytes.TrimSpace(message)
		switch string(bytes.ToLower(normMsg)) {
		case "?":
			fmt.Fprintln(b.w, b.helpText)
			continue
		case "??", "proto":
			fmt.Fprintln(b.w, b.protoText)
			continue
		}

		if err := b.validate(normMsg); err != nil {
			fmt.Println(err)
			continue
		}

		return normMsg, nil
	}
}

func (b *msgBuffer) ReadMessages() ([][]byte, error) {
	if b.opts == nil || b.opts.reader == nil {
		return nil, errors.New("no msg reader is configured")
	}

	msg, err := b.ReadMessage()
	if err != nil {
		return nil, err
	}

	buf := [][]byte{msg}

	for {
		msg, err := b.ReadMessage(WithReadLinePrompt(b.nextPrompt))
		if err == ErrInterruptTerm {
			return nil, ErrInterruptTerm
		}

		// Ctrl+D will trigger io.EOF if the line is empty
		// it means no new messages are expected
		if err == io.EOF {
			fmt.Println()
			return buf, nil
		}

		if err != nil {
			return nil, err
		}

		buf = append(buf, msg)
	}
}

func (b *msgBuffer) validate(msg []byte) error {
	if b.opts.msgFormat == caller.Text {
		return b.validateText(msg)
	}

	return b.validateJSON(msg)
}

func (b *msgBuffer) validateText(msgTxt []byte) error {
	msg := dynamic.NewMessage(b.opts.messageDesc)
	return msg.UnmarshalText(msgTxt)
}

func (b *msgBuffer) validateJSON(msgJSON []byte) error {
	if len(msgJSON) == 0 {
		return errors.New("syntax error: please provide valid json")
	}

	msg := dynamic.NewMessage(b.opts.messageDesc)
	err := msg.UnmarshalJSON(msgJSON)
	errFmt := "invalid message: %w"
	if err == io.ErrUnexpectedEOF || err == io.EOF {
		errFmt = "syntax error: %w"
	}
	if err != nil {
		return fmt.Errorf(errFmt, err)
	}
	return nil
}

func fieldNames(messageDesc *desc.MessageDescriptor) []string {
	fields := map[string]struct{}{}

	walker := caller.NewFieldWalker()
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

func getMessageDefaults(messageDesc *desc.MessageDescriptor) string {
	msg := dynamicpb.NewMessage(messageDesc.UnwrapMessage())
	msgJSON, _ := protojson.MarshalOptions{
		EmitDefaultValues: true,
		UseProtoNames:     true,
	}.Marshal(msg)

	return string(msgJSON)
}

func protoString(messageDesc *desc.MessageDescriptor) string {
	p := protoprint.Printer{
		Compact: true,
	}
	str, err := p.PrintProtoToString(messageDesc)
	if err != nil {
		str = fmt.Sprintf("error printing proto: %v", err)
	}
	return str
}
