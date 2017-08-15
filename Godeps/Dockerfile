FROM golang

COPY Godeps.json /go/src/bargain/liquefy/Godeps/Godeps.json

ENV GOPATH=/go

WORKDIR /go/src/bargain/liquefy
RUN go get github.com/tools/godep
RUN /go/bin/godep restore

RUN go install -v github.com/Sirupsen/logrus
RUN go install -v github.com/gogo/protobuf/protoc-gen-gogo/descriptor
RUN go install -v github.com/gogo/protobuf/gogoproto
RUN go install -v github.com/lib/pq/hstore
RUN go install -v github.com/qor/inflection
RUN go install -v github.com/jinzhu/gorm
RUN go install -v github.com/lib/pq/oid
RUN go install -v github.com/lib/pq
RUN go install -v github.com/vaughan0/go-ini
RUN go install -v github.com/aws/aws-sdk-go/aws/awserr
RUN go install -v github.com/aws/aws-sdk-go/aws/credentials
RUN go install -v github.com/aws/aws-sdk-go/aws
RUN go install -v github.com/aws/aws-sdk-go/aws/client/metadata
RUN go install -v github.com/aws/aws-sdk-go/aws/awsutil
RUN go install -v github.com/aws/aws-sdk-go/aws/request
RUN go install -v github.com/aws/aws-sdk-go/aws/client
RUN go install -v github.com/aws/aws-sdk-go/aws/corehandlers
RUN go install -v github.com/aws/aws-sdk-go/aws/ec2metadata
RUN go install -v github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds
RUN go install -v github.com/aws/aws-sdk-go/private/endpoints
RUN go install -v github.com/aws/aws-sdk-go/aws/defaults
RUN go install -v github.com/aws/aws-sdk-go/aws/session
RUN go install -v github.com/aws/aws-sdk-go/private/protocol/query/queryutil
RUN go install -v github.com/aws/aws-sdk-go/private/protocol/xml/xmlutil
RUN go install -v github.com/aws/aws-sdk-go/private/protocol/ec2query
RUN go install -v github.com/aws/aws-sdk-go/private/protocol/rest
RUN go install -v github.com/aws/aws-sdk-go/private/signer/v4
RUN go install -v github.com/aws/aws-sdk-go/service/ec2
RUN go install -v github.com/aws/aws-sdk-go/private/protocol/query
RUN go install -v github.com/aws/aws-sdk-go/service/iam
RUN go install -v github.com/influxdb/influxdb/pkg/escape
RUN go install -v github.com/influxdb/influxdb/models
RUN go install -v github.com/influxdb/influxdb/client/v2
RUN go install -v golang.org/x/crypto/curve25519
RUN go install -v golang.org/x/crypto/ssh
RUN go install -v github.com/golang/glog
RUN go install -v github.com/mesos/mesos-go/mesosutil
RUN go install -v github.com/mesos/mesos-go/upid
RUN go install -v github.com/mesos/mesos-go/auth/callback
RUN go install -v golang.org/x/net/context
RUN go install -v github.com/mesos/mesos-go/auth
RUN go install -v github.com/mesos/mesos-go/auth/sasl/mech
RUN go install -v github.com/mesos/mesos-go/mesosutil/process
RUN go install -v github.com/stretchr/objx
RUN go install -v github.com/stretchr/testify/assert
RUN go install -v github.com/stretchr/testify/mock
RUN go install -v github.com/mesos/mesos-go/messenger
RUN go install -v github.com/mesos/mesos-go/auth/sasl
RUN go install -v github.com/mesos/mesos-go/auth/sasl/mech/crammd5
RUN go install -v github.com/mesos/mesos-go/detector
RUN go install -v github.com/samuel/go-zookeeper/zk
RUN go install -v github.com/mesos/mesos-go/detector/zoo
RUN go install -v github.com/pborman/uuid
RUN go install -v github.com/mesos/mesos-go/scheduler
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/parsers
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/ulimit
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/volume
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/opts
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/Sirupsen/logrus
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/fileutils
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/units
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/system
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/idtools
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/ioutils
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/pools
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/promise
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/archive
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/opencontainers/runc/libcontainer/user
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/homedir
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/stdcopy
RUN go install -v github.com/fsouza/go-dockerclient/external/github.com/hashicorp/go-cleanhttp
RUN go install -v github.com/fsouza/go-dockerclient
RUN go install -v github.com/mesos/mesos-go/executor
RUN go install -v github.com/dgrijalva/jwt-go
RUN go install -v github.com/golang/protobuf/proto
RUN go install -v gopkg.in/go-playground/validator.v8
RUN go install -v github.com/gin-gonic/gin/binding
RUN go install -v github.com/gin-gonic/gin/render
RUN go install -v github.com/manucorporat/sse
RUN go install -v github.com/gin-gonic/gin
RUN go install -v golang.org/x/crypto/blowfish
RUN go install -v golang.org/x/crypto/bcrypt
RUN go install -v github.com/mattbaird/elastigo/lib