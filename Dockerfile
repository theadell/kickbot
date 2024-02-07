FROM golang:1.22.0-alpine3.19 as builder

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY go.mod go.sum ./
RUN go mod download 

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o kickbot ./cmd/kickbot/

FROM alpine:3.19.1

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/kickbot /kickbot

ENTRYPOINT ["/kickbot"]

EXPOSE 4000
