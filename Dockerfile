# syntax=docker/dockerfile:1.7

# ---- build stage ---------------------------------------------------------
FROM golang:1.23-alpine AS build
WORKDIR /src

# Cache module downloads in a separate layer.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .

# Build a statically-linked, stripped binary.
ARG VERSION=dev
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/server \
    ./cmd/server

RUN go build \
    -ldflags="-s -w" \
    -o /out/migrate \
    ./cmd/migrate

# ---- runtime stage -------------------------------------------------------
# Distroless ships with no shell, no package manager — minimal attack surface.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/migrate /app/migrate

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
