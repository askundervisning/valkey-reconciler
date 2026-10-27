# syntax=docker/dockerfile:1

FROM golang:1.27.3-alpine3.24@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o valkey-reconciler main.go

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
WORKDIR /app
COPY --from=builder /app/valkey-reconciler /app/valkey-reconciler
RUN chmod +x /app/valkey-reconciler
EXPOSE 8080
CMD ["/app/valkey-reconciler"] 
