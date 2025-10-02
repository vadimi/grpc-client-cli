#!/bin/bash
root=$(git rev-parse --show-toplevel)

localbin="$root/.bin"

os="linux-x86_64"
if [[ "$OSTYPE" == "darwin"* ]]; then
    os="osx-x86_64"
fi

# check that protoc compiler exists and download it if required
PROTOBUF_VERSION=32.1
PROTOC_FILENAME=protoc-${PROTOBUF_VERSION}-${os}.zip
PROTOC_PATH=$localbin/protoc-$PROTOBUF_VERSION
if [ ! -d $PROTOC_PATH ] ; then
    mkdir -p $PROTOC_PATH
    curl -L https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOBUF_VERSION}/${PROTOC_FILENAME} > $localbin/$PROTOC_FILENAME
    mkdir -p $PROTOC_PATH
    unzip -o $localbin/$PROTOC_FILENAME -d $localbin/protoc-$PROTOBUF_VERSION
    rm $localbin/$PROTOC_FILENAME
fi

# it gets the version of protoc-gen-go from go.mod file
protoc_gen_go_grpc_plugin="$(go tool -n protoc-gen-go-grpc)"
protoc_gen_go_grpc_plugin="$(go tool -n protoc-gen-go-grpc)"
protoc_gen_go_plugin="$(go tool -n protoc-gen-go)"
protoc_gen_go_plugin="$(go tool -n protoc-gen-go)"

PATH=$PATH:$localbin $PROTOC_PATH/bin/protoc \
  --plugin=protoc-gen-go-grpc="${protoc_gen_go_grpc_plugin}" \
  --plugin=protoc-gen-go="${protoc_gen_go_plugin}" \
  --go_out=$root/internal/testing/grpc_testing --go-grpc_out=require_unimplemented_servers=false:$root/internal/testing/grpc_testing -I"$root/testdata" $root/testdata/test.proto

