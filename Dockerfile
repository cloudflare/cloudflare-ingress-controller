FROM golang:1.11-alpine AS build
WORKDIR /project

ARG VERSION="unknown"
ARG GO111MODULE="on"
ARG GO_EXTLINK_ENABLED="0"
ARG CGO_ENABLED="0"
ARG GOOS="linux"
ARG GOARCH="amd64"

COPY . /project

RUN apk add --no-cache upx ca-certificates git && \
	go mod download && \
	go build \
    	-o /go/bin/argot \
    	-ldflags="-d -w -s -extldflags -static -X main.version=${VERSION}" \
    	-tags netgo -installsuffix netgo \
    	-v cmd/argot/main.go && \
    upx --coff --brute --no-progress /go/bin/argot

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/argot /bin/argot
