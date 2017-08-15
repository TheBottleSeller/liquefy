FROM liquify-deps

ENV GOPATH=/go

COPY ./ /go/src/bargain/liquefy

WORKDIR /go/src/bargain/liquefy

RUN go install -v bargain/liquefy/common
RUN go install -v bargain/liquefy/models
RUN go install -v bargain/liquefy/db
RUN go install -v bargain/liquefy/awsutil
RUN go install -v bargain/liquefy/cloudprovider
RUN go install -v bargain/liquefy/influxdb