FROM golang:alpine3.6 AS binary
ADD . /app
WORKDIR /app
RUN apk update && \
    apk upgrade && \
    apk add git
RUN CGO_ENABLED=0 go build -o rpmserver

FROM scratch
MAINTAINER Andreas Krey <a.krey@gmx.de>

WORKDIR /data

COPY --from=binary /app/rpmserver /rpmserver

EXPOSE 4040

VOLUME ["/data"]
CMD ["/rpmserver"]
