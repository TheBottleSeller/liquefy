FROM liquefy-common

COPY ./ /go/src/bargain/liquefy/executor

WORKDIR /root

RUN go build -v -o /root/executor /go/src/bargain/liquefy/executor/entrypoint/executor.go
RUN go build -v -o /root/executorServer /go/src/bargain/liquefy/executor/entrypoint/executorServer.go
