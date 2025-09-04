# Stage 1: Build the Go binary
FROM golang:1.24 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files and download dependencies
COPY producer_service/go.mod producer_service/go.sum producer_service/main.go ./
COPY message /message

RUN go mod download

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o producer_service ./main.go

# Stage 2: Create a minimal image with the binary
FROM alpine:latest

# Set the working directory inside the container
WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/producer_service .

# Expose the port your application listens on (if applicable)
EXPOSE 8080

# Command to run the application
CMD ["./producer_service"]