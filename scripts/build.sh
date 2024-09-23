#!/usr/bin/env bash

# build protobuf
function build_protobuf() {
    protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/calculator.proto
}

# build go binary
function build_go_binary() {
    go build -o instance
}

# build all
function build_all() {
    build_protobuf
    build_go_binary
}

