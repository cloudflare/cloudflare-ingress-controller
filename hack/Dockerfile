FROM golang:1.11.2 AS build
WORKDIR /go/src/github.com/cloudflare/cloudflare-ingress-controller

RUN go get github.com/golang/dep/cmd/dep
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure -v -vendor-only

COPY cmd cmd
COPY internal internal
RUN GO_EXTLINK_ENABLED=0 CGO_ENABLED=0 GOOS=ARG_OS GOARCH=ARG_ARCH GOARM=ARG_ARM go build \
    -o /go/bin/argot \
    -ldflags="-w -s -extldflags -static -X main.version=ARG_VERSION" \
    -tags netgo -installsuffix netgo \
    -v github.com/cloudflare/cloudflare-ingress-controller/cmd/argot

FROM ARG_ROOT/alpine:3.8 AS final
RUN apk --no-cache add ca-certificates
COPY --from=build /go/bin/argot /bin/argot