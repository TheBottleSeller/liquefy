FROM liquefy-common

COPY ./ /go/src/bargain/liquefy/api

COPY ./entrypoint/docs /root/
COPY ./entrypoint/swagger /root/

RUN go build -v -o /root/apiServer /go/src/bargain/liquefy/api/entrypoint/apiService.go
