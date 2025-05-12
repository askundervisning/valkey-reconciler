# syntax=docker/dockerfile:1

FROM golang:1.21-alpine AS builder
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download


COPY . .
RUN go build -o test-server main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/test-server /app/test-server
RUN chmod +x /app/test-server
EXPOSE 8080
CMD ["/app/test-server"] 