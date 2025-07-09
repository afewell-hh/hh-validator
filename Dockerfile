FROM ubuntu:22.04

# Set environment variables
ENV DEBIAN_FRONTEND=noninteractive
ENV PATH="/usr/local/bin:$PATH"

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install oras
RUN curl -fsSL https://i.hhdev.io/oras | bash

# Install hhfab
RUN curl -fsSL https://i.hhdev.io/hhfab | bash

# Verify installations
RUN oras version
RUN hhfab --version

# Create app directory
WORKDIR /app

# Copy the Go binary (will be built in CI/CD or locally)
COPY server/validator-server /app/validator-server

# Create directory for temporary file operations
RUN mkdir -p /tmp/validator-workdir

# Set permissions
RUN chmod +x /app/validator-server

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the application
CMD ["/app/validator-server"]