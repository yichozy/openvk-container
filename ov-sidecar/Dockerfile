FROM golang:1.26.1 as builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
COPY cache/ ./cache/
COPY config/ ./config/
COPY handler/ ./handler/
COPY openviking/ ./openviking/
COPY sync/ ./sync/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o ov-sidecar .

FROM alpine:3.20

RUN apk add --no-cache ripgrep pcre2 rsync

WORKDIR /app
COPY --from=builder /build/ov-sidecar .

RUN mkdir -p /data

EXPOSE 1935

ENTRYPOINT ["/app/ov-sidecar"]
