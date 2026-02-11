# Build Stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 for static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o apigatewayplane cmd/server/main.go

# Run Stage
FROM alpine:latest

WORKDIR /app

# Install certificates for HTTPS (if needed outbound)
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/apigatewayplane .

# Expose port
EXPOSE 8080

# Run
CMD ["./apigatewayplane"]
