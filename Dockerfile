FROM golang:1.19-alpine3.16 as build
WORKDIR /go/app

COPY go.mod ./
COPY go.sum ./

RUN go mod download
COPY . /go/app

RUN go build -o /go/app/pwdchecker /go/app/cmd/pwd-checker/main.go

FROM alpine:3.16
COPY --from=build /go/app/pwdchecker /usr/bin/pwdchecker

RUN apk --no-cache --upgrade --latest add ca-certificates

# set up nsswitch.conf for Go's "netgo" implementation
# - https://github.com/golang/go/blob/go1.9.1/src/net/conf.go#L194-L275
RUN [ ! -e /etc/nsswitch.conf ] && echo 'hosts: files dns' > /etc/nsswitch.conf

ARG USER=checker
RUN addgroup -S $USER; \
    adduser -S $USER -G $USER -D -H -s /bin/nologin

RUN mkdir -p /etc/pwned \
    chown $USER:$USER /etc/pwned

VOLUME /etc/pwned/pwned.gcs

USER $USER

ENTRYPOINT [ "pwdchecker" ]
CMD ["serve"]
