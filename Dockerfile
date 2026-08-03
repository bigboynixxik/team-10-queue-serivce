FROM golang:1.26.4-alpine AS builder

WORKDIR /src
RUN apk add --no-cache ca-certificates git

ARG GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/trajectory-platform".insteadOf "https://github.com/trajectory-platform"

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/auth-service ./cmd/api/main.go

FROM alpine:3.21

WORKDIR /app
RUN apk add --no-cache ca-certificates git

COPY --from=builder /out/auth-service /app/auth-service

EXPOSE 50052

CMD ["./auth-service"]