FROM golang:1.19-alpine3.16 as build
WORKDIR /go/app

COPY go.mod ./
COPY go.sum ./

RUN go mod download
COPY . /go/app

RUN go build -o /go/app/server /go/app/cmd/pwd-checker/server.go

FROM alpine:3.16
COPY --from=build /go/app/server /server

ARG USER=default
RUN adduser -D $USER \
        && chown $USER /server
USER $USER

ENTRYPOINT [ "/server" ]
