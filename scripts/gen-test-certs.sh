#!/bin/bash

set -e

cwd=$(dirname "$(realpath $0)")
localbin=$(readlink -f $cwd/../.bin)
echo $localbin

# check that protoc compiler exists and download it if required
CERTSTRAP_VERSION=1.2.0
CERTSTRAP_PATH=$localbin/certsrtap-$CERTSTRAP_VERSION
if [ ! -d $CERTSTRAP_PATH ] ; then
    mkdir -p $CERTSTRAP_PATH
    curl -L https://github.com/square/certstrap/releases/download/v${CERTSTRAP_VERSION}/certstrap-${CERTSTRAP_VERSION}-linux-amd64 > $CERTSTRAP_PATH/certstrap
    chmod +x $CERTSTRAP_PATH/certstrap
fi

export PATH=$PATH:$CERTSTRAP_PATH

function cert() {
  certstrap --depot-path $cwd/../testdata/certs "$@" 
}

# CA
cert init --years 10 --common-name test_ca -passphrase ""

# server
cert request-cert --common-name test_server --ip 127.0.0.1 --domain localhost -passphrase ""
cert sign test_server --years 10 --CA test_ca 

# client
cert request-cert --common-name test_client -passphrase ""
cert sign test_client --years 10 --CA test_ca 

# other CA
cert init --years 10 --common-name other_ca -passphrase ""

# other client
cert request-cert --common-name other_client -passphrase ""
cert sign other_client --years 10 --CA other_ca 
