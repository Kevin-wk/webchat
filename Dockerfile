FROM golang:latest

MAINTAINER Card "445864742@qq.com"

WORKDIR $GOPATH/src/webchat
ADD . $GOPATH/src/webchat

RUN go build .

EXPOSE 8889

ENTRYPOINT ["./webchat"]
