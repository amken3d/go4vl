# Start with a base image that includes Go.
FROM golang:latest

# Set the working directory inside the container.
WORKDIR /app

# Copy the Go Modules manifests and fetch any dependencies.
# (If you're not using Go Modules, you can skip this step.)
COPY go.mod go.sum ./
RUN go mod download

# Copy the local package files to the container's workspace.
COPY . .

# Install v4l2loopback using the package manager.
RUN apt-get update && apt-get install -y \
    v4l2loopback-dkms \
    v4l2loopback-utils

# Load the v4l2loopback module.
# Note: This might not be effective in some Docker environments and may require additional host configurations.
RUN modprobe v4l2loopback

# Run tests for the Go modules.
# Here, 'go test ./...' will recursively run all tests in all subdirectories.
RUN go test ./...

# The default command can be a no-op, as the primary purpose of this Dockerfile is to run tests.
CMD ["tail", "-f", "/dev/null"]