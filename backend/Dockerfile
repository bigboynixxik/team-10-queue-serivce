FROM golang:1.26.4-alpine AS builder

WORKDIR /src
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/queue-service ./cmd/api/main.go

FROM alpine:3.21

ARG APP_PORT=8080
ENV APP_PORT=${APP_PORT}

WORKDIR /app
RUN apk add --no-cache ca-certificates git

COPY --from=builder out/queue-service /app/queue-service

EXPOSE ${APP_PORT}

CMD ["./queue-service"]