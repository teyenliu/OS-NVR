#!/bin/sh

set -e

osnvr_version="v0.1.0"

FILE=./go1.18.5.linux-amd64.tar.gz
if test -f "$FILE"; then
    echo "$FILE exists."
else 
    wget https://go.dev/dl/go1.18.5.linux-amd64.tar.gz -P ./
fi
go build -o nvr ../start/build/nvr.go 
cp -rf ../web ./web
docker build --build-arg osnvr_version=$osnvr_version -t itri-os-nvr:latest -f ./Dockerfile .
docker tag itri-os-nvr:latest 140.96.113.140:5000/itri-os-nvr:latest

