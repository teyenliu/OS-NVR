#FROM alpine AS release
#FROM golang:1.18-alpine3.15
FROM ubuntu:18.04

LABEL maintainer="Danny Liu. mail: danny.liu@itri.org.tw"

ARG osnvr_version

# ensure local python is preferred over distribution python
ENV PATH /usr/local/bin:/usr/bin:$PATH

# http://bugs.python.org/issue19846
# > At the moment, setting "LANG=C" on a Linux system *fundamentally breaks Python 3*
# > and that's kinda not OK
ENV LANG C.UTF-8

# ensure annoying packages like `tzdata` won't ask a lot of questions
ARG DEBIAN_FRONTEND=noninteractive

RUN apt update && \
    apt install -y ffmpeg sed gcc musl-dev sudo git \
        ca-certificates vim wget

WORKDIR /app

# Install golang
COPY ./bundle/go1.18.5.linux-amd64.tar.gz /tmp/go1.18.5.linux-amd64.tar.gz
RUN cd /tmp && \
    tar -xvf go1.18.5.linux-amd64.tar.gz && \
    mv go /usr/local && \
    rm -rf /tmp/go* && \
    mkdir /go
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$GOROOT/bin:/usr/local/lib:$PATH


#RUN git clone https://github.com/hybridgroup/gocv.git && \
#    cd gocv && \
#    sed 's/sudo //' Makefile > NewMakefile && \
#    mv NewMakefile Makefile && \
#    make install

# Install ITRI's OS-NVR
COPY *.go ./
COPY go.* ./
#COPY .env ./
COPY addons ./addons
COPY pkg ./pkg
COPY start ./start
COPY utils ./utils
COPY configs/templ ./configs
COPY web ./web
RUN mkdir storage
RUN go mod tidy

EXPOSE 2020 2021 2022

CMD ["go", "run", "start/start.go", "-env", "configs/env.yaml"]
