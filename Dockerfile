# Use an official Golang image for the build stage
FROM golang:1.23.1 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy Go Modules files first and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go app for a Linux environment
RUN CGO_ENABLED=0 GOOS=linux go build -o myapp ./cmd/main.go

# Use a minimal image for the final stage
FROM alpine:latest

# Install necessary certificates if your app needs to communicate over HTTPS
RUN apk --no-cache add ca-certificates

# Set the working directory inside the minimal image
WORKDIR /app

# Copy the compiled Go binary and environment file from the builder stage
COPY --from=builder /app/myapp .
COPY --from=builder /app/.env .

# Copy the email template
COPY --from=builder /app/api/email/template.html ./api/email/

# Expose necessary ports
EXPOSE 1234
EXPOSE 2345

# Start the Go application
CMD ["./myapp"]
