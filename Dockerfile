FROM golang:alpine3.6 AS binary
ADD . /app
WORKDIR /app
RUN apk update && \
    apk upgrade && \
    apk add git
RUN CGO_ENABLED=0 go build -o rpmserver

FROM centos:centos7
MAINTAINER Andreas Krey <a.krey@gmx.de>

WORKDIR /data

RUN yum install -y createrepo

COPY --from=binary /app/rpmserver /rpmserver

EXPOSE 8080

VOLUME ["/data"]
CMD ["/rpmserver"]
