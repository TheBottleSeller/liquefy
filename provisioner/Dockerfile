FROM liquefy-common

COPY ./ /go/src/bargain/liquefy/provisioner

WORKDIR /root

RUN go build -v -o /root/provisionerService /go/src/bargain/liquefy/provisioner/entrypoint/provisionerService.go
