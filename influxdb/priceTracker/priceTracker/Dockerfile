FROM debian:jessie
MAINTAINER nbatlivala <neil.batlivala@gmail.com>

RUN apt-get update && apt-get install -y curl
ENV TELEGRAF_VERSION 0.2.4
RUN curl -s -o /tmp/telegraf_latest_amd64.deb http://get.influxdb.org/telegraf/telegraf_${TELEGRAF_VERSION}_amd64.deb && \
  dpkg -i /tmp/telegraf_latest_amd64.deb && \
  rm /tmp/telegraf_latest_amd64.deb && \
  rm -rf /var/lib/apt/lists/*

COPY telegraf.toml /config/telegraf.toml
COPY spotPriceTracker /liquefy/spotPriceTracker
COPY run.sh /run.sh
RUN chmod u+x /liquefy/spotPriceTracker
RUN chmod u+x /run.sh

ENV INFLUXDB_HOST localhost

CMD ["/run.sh"]
