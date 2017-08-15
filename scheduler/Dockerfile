FROM liquefy-common

COPY ./ /go/src/bargain/liquefy/scheduler

RUN go build -v -o /root/schedulerService /go/src/bargain/liquefy/scheduler/entrypoint/schedulerService.go
