FROM golang:1.10.8 AS build
WORKDIR /go/src/github.com/cloudflare/cloudflare-ingress-controller

ARG VERSION="unknown"

RUN go get github.com/golang/dep/cmd/dep
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure -v -vendor-only

COPY cmd cmd
COPY internal internal
RUN GO_EXTLINK_ENABLED=0 CGO_ENABLED=0 GOOS=linux go build \
    -o /go/bin/argot \
    -ldflags="-w -s -extldflags -static -X main.version=${VERSION}" \
    -tags netgo -installsuffix netgo \
    -v github.com/cloudflare/cloudflare-ingress-controller/cmd/argot

FROM alpine:3.9 AS final
RUN apk --no-cache add ca-certificates
COPY --from=build /go/bin/argot /bin/argot