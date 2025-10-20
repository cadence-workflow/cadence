# Build stage
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Copy source code
COPY hello-world/main.go .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /hello-world main.go

# Final stage
FROM alpine:3.18

# Copy the binary from builder
COPY --from=builder /hello-world /usr/local/bin/

# Set the binary as the entrypoint
ENTRYPOINT ["hello-world"]

# Install necessary runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Copy the built executable
COPY --from=builder /samples/hello-world /app/hello-world

# Set working directory
WORKDIR /app

# Command to run the sample
ENTRYPOINT ["/app/hello-world"]