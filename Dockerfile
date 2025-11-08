# syntax=docker/dockerfile:1

FROM golang:1.24 AS base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM base AS server-build
RUN CGO_ENABLED=0 GOOS=linux go build -o termchat-server ./cmd/server

FROM base AS client-build
RUN CGO_ENABLED=0 GOOS=linux go build -o termchat-client ./cmd/client

FROM debian:bookworm-slim AS client
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=client-build /app/termchat-client /usr/local/bin/termchat-client
ENV TERM=xterm-256color
ENTRYPOINT ["termchat-client"]

FROM gcr.io/distroless/static-debian12 AS server
COPY --from=server-build /app/termchat-server /termchat-server
ENV TERMCHAT_ADDR=:8080
ENV TERMCHAT_PATH=/join
EXPOSE 8080
ENTRYPOINT ["/termchat-server"]
