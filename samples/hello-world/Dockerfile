# Build stage
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the samples
RUN CGO_ENABLED=0 GOOS=linux go build -o /samples/hello-world ./cmd/samples/hello-world

# Final stage
FROM alpine:3.18

# Install necessary runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Copy the built executable
COPY --from=builder /samples/hello-world /app/hello-world

# Set working directory
WORKDIR /app

# Command to run the sample
ENTRYPOINT ["/app/hello-world"]