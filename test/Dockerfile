FROM golang

EXPOSE 80
COPY testServer.go /go/src/testServer.go
ENTRYPOINT printenv && go run /go/src/testServer.go