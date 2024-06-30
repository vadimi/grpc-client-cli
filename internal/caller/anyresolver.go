package caller

import (
	"errors"

	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/builder"
	"github.com/jhump/protoreflect/dynamic"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/runtime/protoiface"
	"google.golang.org/protobuf/types/dynamicpb"
)

var customAnyDescr *desc.MessageDescriptor

func init() {
	md, err := builder.NewMessage("any").
		AddField(builder.NewField("value", builder.FieldTypeBytes())).
		AddField(builder.NewField("err", builder.FieldTypeString())).
		Build()
	if err != nil {
		panic(err)
	}

	customAnyDescr = md
}

// anyResolver resolves types specified in typeURL/@type field of google.protobuf.Any message
// or falls back to the one that just represents Any message with an error field
type anyResolver struct {
	fdescCache *FileDescCache
}

type anyResolver2 struct {
	*dynamicpb.Types
}

func NewRes(fdescCache *FileDescCache) *anyResolver2 {
	ff := &protoregistry.Files{}
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		ff.RegisterFile(fd)
		return true
	})
	if fdescCache != nil {
		for _, f := range fdescCache.Files() {
			ff.RegisterFile(f.UnwrapFile())
		}
	}
	return &anyResolver2{dynamicpb.NewTypes(ff)}
}

func (t *anyResolver2) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	mt, err := t.Types.FindMessageByURL(url)
	if err != nil {
		if errors.Is(err, protoregistry.NotFound) {
			mmm := dynamicpb.NewMessage(customAnyDescr.UnwrapMessage())
			tt := &unknownMsgType{mmm}
			return tt, nil
		}
		return mt, err
	}
	return mt, nil
}

func (a *anyResolver) Resolve(typeURL string) (proto.Message, error) {
	files := []*desc.FileDescriptor{}
	if a.fdescCache != nil {
		files = a.fdescCache.Files()
	}
	baseResolver := dynamic.AnyResolver(dynamic.NewMessageFactoryWithDefaults(), files...)
	m, err := baseResolver.Resolve(typeURL)
	if err == nil {
		return m, nil
	}
	return &anyWrapper{dynamic.NewMessage(customAnyDescr)}, nil
}

type unknownMsgType struct {
	m *dynamicpb.Message
}

func (u *unknownMsgType) New() protoreflect.Message {
	return &anyWrapper2{u.m}
}

func (u *unknownMsgType) Zero() protoreflect.Message {
	return &anyWrapper2{u.m}
}

func (u *unknownMsgType) Descriptor() protoreflect.MessageDescriptor {
	return u.m.Descriptor()
}

type anyWrapper2 struct {
	*dynamicpb.Message
}

func (m *anyWrapper2) Interface() protoreflect.ProtoMessage {
	return m
}

func (m *anyWrapper2) ProtoReflect() protoreflect.Message {
	return m
}

func (a *anyWrapper2) ProtoMethods() *protoiface.Methods {
	return &protoiface.Methods{
		Unmarshal: func(in protoiface.UnmarshalInput) (protoiface.UnmarshalOutput, error) {
			v := in.Message.(*anyWrapper2)
			v.Set(v.Descriptor().Fields().ByName("err"), protoreflect.ValueOfString("type not found"))
			return protoiface.UnmarshalOutput{}, nil
		},
		Flags: protoiface.SupportMarshalDeterministic,
	}
}

var _ protoreflect.ProtoMessage = (*anyWrapper2)(nil)

type anyWrapper struct {
	*dynamic.Message
}

func (a *anyWrapper) Unmarshal(b []byte) error {
	a.SetFieldByName("value", b)
	a.SetFieldByName("err", "type not found")
	return nil
}

var _ proto.Message = (*anyWrapper)(nil)
