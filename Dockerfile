FROM --platform=linux/amd64 golang:1.21-alpine as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o kickbot

FROM --platform=linux/amd64 alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/kickbot /kickbot

ENTRYPOINT ["/kickbot"]

EXPOSE 4000
