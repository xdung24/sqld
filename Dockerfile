FROM golang:1.24 AS builder

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
FROM debian:trixie-slim AS production
# Copy the binary to the production image from the builder stage.
COPY --from=builder /app/tmp/sqld.exe /usr/local/bin/sqld

# test the binary
RUN sqld --version

# Entrypoint
ENTRYPOINT ["/usr/local/bin/sqld"]
