FROM golang:1.18-alpine3.16 as builder

WORKDIR $GOPATH/src/wserver
COPY . .

RUN apk add --no-cache git && set -x && \
    go mod init && go get -d -v
RUN CGO_ENABLED=0 GOOS=linux go build -o /server server.go

FROM alpine:latest

WORKDIR /
COPY --from=builder /server . 

ADD entrypoint.sh /entrypoint.sh

RUN  chmod +x /server   && chmod 777 /entrypoint.sh
ENTRYPOINT  /entrypoint.sh 

EXPOSE 80