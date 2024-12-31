FROM golang:1.23 AS builder

RUN apt-get update -y && apt-get upgrade -y && apt-get install -y make build-essential
# Create and change to the app directory.
WORKDIR /app

# Retrieve application dependencies.
# This allows the container build to reuse cached dependencies.
# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy local code to the container image.
COPY . .

# Build the binary.
RUN mkdir -p tmp && sh build.sh && ls -lah /app/tmp

# Use the official Debian slim image for a production container.
# https://hub.docker.com/_/debian
FROM debian:bookworm-slim AS production

RUN apt-get update -y && apt-get upgrade -y && apt-get install -y ca-certificates bash curl wget && rm -rf /var/lib/apt/lists/*

# Copy the binary to the production image from the builder stage.
COPY --from=builder /app/tmp/sqld.exe /usr/local/bin/sqld

# Entrypoint
ENTRYPOINT ["/usr/local/bin/sqld"]
