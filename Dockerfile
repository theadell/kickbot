FROM --platform=linux/amd64 golang:1.21-alpine as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o kickbot .

FROM --platform=linux/amd64 alpine:latest

# Install CA certificates, necessary for making HTTPS requests
RUN apk --no-cache add ca-certificates

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/kickbot /kickbot

# Set the binary as the entrypoint of the container
ENTRYPOINT ["/kickbot"]

# Expose port (the same as in your application)
EXPOSE 4000
