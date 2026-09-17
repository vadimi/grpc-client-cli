package app

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoprint"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// ValidateMessage checks that msg is a valid message of messageDesc in the
// given format. The returned error is suited for display to the user.
func ValidateMessage(msg []byte, messageDesc protoreflect.MessageDescriptor, format caller.MsgFormat) error {
	if format == caller.Text {
		return validateText(msg, messageDesc)
	}

	return validateJSON(msg, messageDesc)
}

func validateText(msgTxt []byte, messageDesc protoreflect.MessageDescriptor) error {
	msg := dynamicpb.NewMessage(messageDesc)
	return prototext.Unmarshal(msgTxt, msg)
}

func validateJSON(msgJSON []byte, messageDesc protoreflect.MessageDescriptor) error {
	if len(msgJSON) == 0 {
		return errors.New("syntax error: please provide valid json")
	}

	msg := dynamicpb.NewMessage(messageDesc)
	err := protojson.Unmarshal(msgJSON, msg)
	errFmt := "invalid message: %w"
	if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
		errFmt = "syntax error: %w"
	}
	if err != nil {
		return fmt.Errorf(errFmt, err)
	}

	return nil
}

// FieldNames returns the sorted field names reachable from messageDesc,
// used for editor completion.
func FieldNames(messageDesc protoreflect.MessageDescriptor) []string {
	fields := map[string]struct{}{}

	walker := caller.NewFieldWalker()
	walker.Walk(messageDesc, func(f protoreflect.FieldDescriptor) {
		fields[string(f.Name())] = struct{}{}
	})

	names := slices.Collect(maps.Keys(fields))
	slices.Sort(names)
	return names
}

// MessageDefaults returns the default message of messageDesc in json form,
// shown as an editing aid.
func MessageDefaults(messageDesc protoreflect.MessageDescriptor) string {
	msg := dynamicpb.NewMessage(messageDesc)
	msgJSON, _ := protojson.MarshalOptions{
		EmitDefaultValues: true,
		UseProtoNames:     true,
	}.Marshal(msg)

	return string(msgJSON)
}

// MessageProto returns the compact proto definition of messageDesc.
func MessageProto(messageDesc protoreflect.MessageDescriptor) string {
	wrappedDesc, err := desc.WrapMessage(messageDesc)
	if err != nil {
		return fmt.Sprintf("error wrapping descriptor: %v", err)
	}

	p := protoprint.Printer{
		Compact: true,
	}
	str, err := p.PrintProtoToString(wrappedDesc)
	if err != nil {
		return fmt.Sprintf("error printing proto: %v", err)
	}
	return str
}

// PrintFile writes the proto definition of f to w.
func PrintFile(w io.Writer, f protoreflect.FileDescriptor) error {
	wrappedFile, err := desc.WrapFile(f)
	if err != nil {
		return err
	}
	p := &protoprint.Printer{}
	return p.PrintProtoFile(wrappedFile, w)
}
