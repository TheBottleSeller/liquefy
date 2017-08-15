FROM golang


#TODO : Move to static Build process
COPY ./ /go/src/bargain/liquefy/slave/logging
RUN go get gopkg.in/yaml.v2
WORKDIR /root
RUN go build /go/src/bargain/liquefy/slave/logging/generate_filebeatyml.go


RUN apt-get install -y curl
RUN curl -L -O curl -L -O https://download.elastic.co/beats/filebeat/filebeat_1.0.1_amd64.deb
RUN dpkg -i filebeat_1.0.1_amd64.deb

#Copy the go code and compile
COPY ./start_filebeat.sh /root/start_filebeat.sh

#done
ENTRYPOINT /root/start_filebeat.sh