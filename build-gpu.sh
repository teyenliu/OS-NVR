#!/bin/sh

set -e

osnvr_version="v0.1.0"

FILE=./bundle/go1.18.5.linux-amd64.tar.gz
FFMPEG=./bundle/ffmpeg
NVCODEC=./bundle/nv-codec-headers

if test -f "$FILE"; then
    echo "$FILE exists."
else 
    wget https://go.dev/dl/go1.18.5.linux-amd64.tar.gz -P ./bundle/
fi

if test -d "$FFMPEG"; then
    echo "$FFMPEG exists."
else 
    git clone https://git.ffmpeg.org/ffmpeg.git  ./bundle/ffmpeg
    git clone https://git.videolan.org/git/ffmpeg/nv-codec-headers.git ./bundle/nv-codec-headers
fi

go build -o bundle/nvr start/build/nvr.go 
docker build --build-arg osnvr_version=$osnvr_version -f bundle/Dockerfile-gpu -t itri-os-nvr-gpu:latest .


