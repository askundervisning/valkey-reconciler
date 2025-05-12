# syntax=docker/dockerfile:1

FROM golang:1.21-alpine AS builder
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o valkey-reconciler main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/valkey-reconciler /app/valkey-reconciler
RUN chmod +x /app/valkey-reconciler
EXPOSE 8080
CMD ["/app/valkey-reconciler"] 