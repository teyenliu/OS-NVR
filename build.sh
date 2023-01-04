#!/bin/sh

set -e

osnvr_version="v0.1.0"

FILE=./bundle/go1.18.5.linux-amd64.tar.gz
if test -f "$FILE"; then
    echo "$FILE exists."
else 
    wget https://go.dev/dl/go1.18.5.linux-amd64.tar.gz -P ./bundle/
fi
go build -o bundle/nvr start/build/nvr.go 
docker build --build-arg osnvr_version=$osnvr_version -t itri-os-nvr:latest -f ./bundle/Dockerfile .


