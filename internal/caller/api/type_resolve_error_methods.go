package api

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/runtime/protoiface"
)

func (a *TypeResolveError) ProtoMethods() *protoiface.Methods {
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
		Flags: protoiface.SupportUnmarshalDiscardUnknown,
	}
}
