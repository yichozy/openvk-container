FROM golang:1.26.1 as builder

WORKDIR /build

ENV GO111MODULE=on

RUN go env -w GOPRIVATE=github.com/yichozy

ARG GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}@github.com".insteadOf "https://github.com"

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o grep-server .

FROM alpine:3.20

RUN apk add --no-cache ripgrep

WORKDIR /app
COPY --from=builder /build/grep-server .

RUN mkdir -p /data

EXPOSE 1935

ENTRYPOINT ["/app/grep-server"]
