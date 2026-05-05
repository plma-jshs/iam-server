# Makefile 

init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/authzed/grpcutil/cmd/protoc-gen-grpc-gateway@latest
	go install github.com/authzed/authzed-go/v1
	go mod download
    go mod tidy

generate:
	go generate ./...