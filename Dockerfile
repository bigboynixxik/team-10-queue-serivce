FROM golang:1.26.4-alpine AS builder

WORKDIR /src
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/queue-service ./cmd/api/main.go

FROM alpine:3.21

WORKDIR /app
RUN apk add --no-cache ca-certificates git

COPY --from=builder out/queue-service /app/queue-service

EXPOSE 50052

CMD ["./auth-service"]