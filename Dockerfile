ARG AUTHELIA_VERSION=4.39.19

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24-alpine AS api
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web /src/cmd/server/static ./cmd/server/static
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/authelia-webui ./cmd/server

FROM alpine:3.22 AS authelia
ARG AUTHELIA_VERSION
ARG TARGETARCH
RUN apk add --no-cache ca-certificates curl tar
RUN arch="${TARGETARCH:-$(uname -m)}"; \
    case "$arch" in \
      amd64|x86_64) authelia_arch=amd64 ;; \
      arm64|aarch64) authelia_arch=arm64 ;; \
      *) echo "unsupported architecture: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/authelia/authelia/releases/download/v${AUTHELIA_VERSION}/authelia-v${AUTHELIA_VERSION}-linux-${authelia_arch}.tar.gz" \
      | tar -xz -C /tmp; \
    find /tmp -type f -name authelia -exec cp {} /authelia \;; \
    chmod +x /authelia

FROM alpine:3.22 AS docs
ARG AUTHELIA_VERSION
RUN apk add --no-cache ca-certificates curl tar
RUN curl -fsSL "https://github.com/authelia/authelia/archive/refs/tags/v${AUTHELIA_VERSION}.tar.gz" \
      | tar -xz -C /tmp; \
    mkdir -p /authelia-docs; \
    cp -a "/tmp/authelia-${AUTHELIA_VERSION}/docs/content/integration/openid-connect/clients" /authelia-docs/clients

FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=api /out/authelia-webui /authelia-webui
COPY --from=docs /authelia-docs/clients /authelia-docs/clients
COPY --from=authelia /authelia /usr/local/bin/authelia
ENV LISTEN_ADDR=:8080
ENV AUTHELIA_USERS_FILE=/config/users_database.yml
ENV AUTHELIA_CONFIG_FILE=/config/configuration.yml
ENV AUTHELIA_BINARY=/usr/local/bin/authelia
ENV AUTHELIA_DOCS_CLIENTS_DIR=/authelia-docs/clients
EXPOSE 8080
USER root:root
ENTRYPOINT ["/authelia-webui"]
