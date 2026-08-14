FROM golang:1.26.4-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# One Dockerfile builds both binaries — the API and the AvitoBackend mock share
# the module, so a second file would only duplicate the build steps.
ARG CMD=api

# Empty values make Go build for the builder's own platform, so the image is
# native both on arm64 laptops and on amd64 CI.
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/service ./cmd/${CMD}

FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 app

WORKDIR /app
COPY --from=builder /out/service /app/service

USER app

EXPOSE 8080

CMD ["./service"]
