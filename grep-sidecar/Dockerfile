FROM golang:1.26.1 as builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o grep-server .

FROM alpine:3.20

RUN apk add --no-cache ripgrep pcre2

WORKDIR /app
COPY --from=builder /build/grep-server .

RUN mkdir -p /data

EXPOSE 1935

ENTRYPOINT ["/app/grep-server"]
