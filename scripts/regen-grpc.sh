#!/bin/bash
cwd=$(dirname "$0")

# macos doesn't have readlink installed by default, so we need to replicate it here
# to get the real path that grpc tooling needs
function getRealPath {
    TARGET_FILE=$1
    cd `dirname $TARGET_FILE`
    TARGET_FILE=`basename $TARGET_FILE`

    while [ -L "$TARGET_FILE" ]
    do
        TARGET_FILE=`readlink $TARGET_FILE`
        cd `dirname $TARGET_FILE`
        TARGET_FILE=`basename $TARGET_FILE`
    done

    PHYS_DIR=`pwd -P`
    echo $PHYS_DIR/$TARGET_FILE
}

localbin=$(getRealPath $cwd/../.bin)

os="linux-x86_64"
if [[ "$OSTYPE" == "darwin"* ]]; then
    os="osx-x86_64"
fi

# check that protoc compiler exists and download it if required
PROTOBUF_VERSION=3.15.6
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
GOBIN=$localbin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
GOBIN=$localbin go install google.golang.org/protobuf/cmd/protoc-gen-go

GOOGLE_PROTO_DIR=$PROTOC_PATH/include/google/protobuf

PATH=$PATH:$localbin $PROTOC_PATH/bin/protoc --go_out=$cwd/../internal/testing/grpc_testing --go-grpc_out=require_unimplemented_servers=false:$cwd/../internal/testing/grpc_testing -I$GOOGLE_PROTO_DIR:"$cwd/../testdata" $cwd/../testdata/test.proto

