FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -o fileserver ./cmd/server/main.go

# Use a small image for the final container
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/fileserver .
COPY --from=builder /app/ui ./ui

# Create uploads directory
RUN mkdir -p ./uploads

# Expose the application port
EXPOSE 8080

# Command to run the executable
CMD ["./fileserver", "--port=8080", "--ui=./ui", "--uploads=./uploads"]