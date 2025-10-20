# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache ca-certificates git

# Set working directory
WORKDIR /build/hello-world

# Copy hello-world code
COPY hello-world/ .

# Initialize module
RUN go mod init hello-world && \
    go mod tidy && \
    go mod download

# Build the sample
RUN CGO_ENABLED=0 GOOS=linux go build -o /hello-world .

# Final stage
FROM alpine:3.18

# Install required runtime packages
RUN apk --no-cache add ca-certificates

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