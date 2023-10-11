package caller

import (
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/builder"
	"github.com/jhump/protoreflect/dynamic"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/runtime/protoiface"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
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
	a := anypb.Any{}
	mt, err := t.Types.FindMessageByURL(url)
	if err != nil {
		if errors.Is(err, protoregistry.NotFound) {
			// e := &api.TypeResolveError{}
			// return e.ProtoReflect().Type(), nil
			mmm := &anyWrapper2{dynamicpb.NewMessage(customAnyDescr.UnwrapMessage())}
			tt := mmm.ProtoReflect().Type()
			tt.Descriptor()
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

type anyWrapper2 struct {
	*dynamicpb.Message
}

func (a *anyWrapper2) Reset() {
	fmt.Println("ddddfdfdfd333")
}

func (a *anyWrapper2) Unmarshal(i protoiface.UnmarshalInput) (protoiface.UnmarshalOutput, error) {
	fmt.Println("111")
	return protoiface.UnmarshalOutput{}, errors.New("fdfdfdf")
}

func (a *anyWrapper2) New() protoreflect.Message {
	panic("ddd1")
}

func (a *anyWrapper2) ProtoMethods() *protoiface.Methods {
	fmt.Println("33333")
	return &protoiface.Methods{
		Unmarshal: func(in protoiface.UnmarshalInput) (protoiface.UnmarshalOutput, error) {
			fmt.Println("eeee999999")
			// v := in.Message.(*anyWrapper2)
			// fmt.Println(v)
			// if !ok {
			// 	return protoiface.UnmarshalOutput{}, errors.New("%T does not implement Unmarshal", v)
			// }
			return protoiface.UnmarshalOutput{}, errors.New("dddd333")
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
