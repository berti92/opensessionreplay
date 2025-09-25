# Use multi-stage build
FROM node:18-slim AS node-deps

# Set working directory for node dependencies
WORKDIR /app

# Copy package files
COPY package*.json ./

# Install node dependencies
RUN npm ci --only=production

# Use golang image for building the Go application
FROM golang:1.21-bullseye AS go-build

# Install build dependencies
RUN apt-get update && apt-get install -y gcc libc6-dev libsqlite3-dev && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY main.go ./
COPY recorder.js ./
COPY demo.html ./

# Copy node_modules from previous stage
COPY --from=node-deps /app/node_modules ./node_modules

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o main .

# Final stage - runtime
FROM debian:bullseye-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y ca-certificates sqlite3 wget && rm -rf /var/lib/apt/lists/*

# Create app directory
WORKDIR /app

# Create data directory for database persistence
RUN mkdir -p /app/data

# Copy built application
COPY --from=go-build /app/main .
COPY --from=go-build /app/recorder.js .
COPY --from=go-build /app/demo.html .
COPY --from=go-build /app/node_modules ./node_modules

# Create non-root user
RUN groupadd -g 1001 appgroup && \
    useradd -u 1001 -g appgroup -m -s /bin/bash appuser

# Change ownership of app directory
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Run the application
CMD ["./main"]