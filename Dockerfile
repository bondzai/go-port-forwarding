# Use Go 1.20 on Alpine as the base image
FROM golang:1.20-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy the source code into the container
COPY . .

# Install Git (required for fetching dependencies)
RUN apk --no-cache add git

# Build the Go application
RUN go build -o myapp .

# Expose the port used by your application
EXPOSE 8080

# Run the Go application when the container starts
CMD ["./myapp"]
