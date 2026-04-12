# Agent, worker, and API binaries for docker-compose (see docker-compose.yml).
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/agent ./cmd/agent && \
	CGO_ENABLED=0 go build -o /out/worker ./cmd/worker && \
	CGO_ENABLED=0 go build -o /out/api ./cmd/api

FROM debian:bookworm-slim
# Chromium + libs for go-rod (Built In T3 / future LinkedIn). Binary: /usr/bin/chromium
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
	ca-certificates \
	chromium \
	fonts-liberation \
	&& rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/agent /usr/local/bin/agent
COPY --from=build /out/worker /usr/local/bin/worker
COPY --from=build /out/api /usr/local/bin/api
COPY data ./data
