FROM liquefy-common

COPY ./ /go/src/bargain/liquefy/priceTracker

RUN go build -o /root/spotPriceTracker /go/src/bargain/liquefy/priceTracker/spotPriceTracker.go