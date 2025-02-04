#!/bin/sh

set -e

osnvr_version="v0.1.0"

FILE=./go1.18.5.linux-amd64.tar.gz
FFMPEG=./ffmpeg
NVCODEC=./nv-codec-headers

if test -f "$FILE"; then
    echo "$FILE exists."
else 
    wget https://go.dev/dl/go1.18.5.linux-amd64.tar.gz -P ./
fi

if test -d "$FFMPEG"; then
    echo "$FFMPEG exists."
else 
    git clone https://git.ffmpeg.org/ffmpeg.git  ./ffmpeg
    git clone https://git.videolan.org/git/ffmpeg/nv-codec-headers.git ./nv-codec-headers
fi

go build -o nvr ../start/build/nvr.go
cp -rf ../web ./web
docker build --build-arg osnvr_version=$osnvr_version -f Dockerfile-gpu -t itri-os-nvr-gpu:latest .
docker tag itri-os-nvr-gpu:latest 140.96.113.140:5000/itri-os-nvr-gpu:latest

