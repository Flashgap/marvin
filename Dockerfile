# Build image
FROM golang:1.26-trixie AS build

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy sources
COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/

# Build
RUN go build -o marvin ./cmd/marvin

# Run image
FROM debian:trixie-slim

# Install CA-Certificates for SSL
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates libc6 \
    && update-ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /app/marvin /app/marvin

ENTRYPOINT ["./marvin"]
