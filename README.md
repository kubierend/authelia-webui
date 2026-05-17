# Authelia WebUI

Small standalone admin UI for file-backed Authelia installs.

It edits:

- `users_database.yml`
- `identity_providers.oidc.clients` in `configuration.yml`

It can create, edit, and delete users and OIDC clients. Passwords and client secrets are generated through the Authelia CLI and shown once after creation or rotation.

## Why

Authelia’s file backend is simple and reliable, but hand-editing users, OIDC clients, hashed passwords, and client secrets gets tedious. This app keeps the files as the source of truth and automates the annoying parts.

## Features

- User create, edit, delete, disable, and password reset
- OIDC client create, edit, delete, and secret rotation
- Authelia-generated Argon2 password hashes
- Authelia-generated PBKDF2 OIDC client secrets
- Client presets from Authelia’s OIDC integration docs
- Rendered application setup notes for presets
- Light and dark mode
- Separate container, suitable for Docker Compose and Kubernetes

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `LISTEN_ADDR` | `:8080` | HTTP listen address |
| `AUTHELIA_USERS_FILE` | `/config/users_database.yml` | Authelia users database |
| `AUTHELIA_CONFIG_FILE` | `/config/configuration.yml` | Authelia main configuration |
| `AUTHELIA_BINARY` | `authelia` locally, `/usr/local/bin/authelia` in the image | CLI used to generate hashes and secrets |
| `AUTHELIA_DOCS_CLIENTS_DIR` | `./authelia-src/docs/content/integration/openid-connect/clients` locally, `/authelia-docs/clients` in the image | Authelia OIDC client docs used for presets |

The container image includes the Authelia binary and the OIDC client docs. The Authelia source tree is not vendored in this repository. For local development without the container, either keep a local checkout at `./authelia-src` or set `AUTHELIA_DOCS_CLIENTS_DIR` to another copy of the docs.

## Docker Compose

```sh
docker compose up
```

The included compose file uses `kubierend/authelia-webui:latest` and mounts `./example/authelia` to `/config`. Replace that mount with the directory that contains your Authelia files.

The container runs as root because many Authelia deployments have root-owned config files. Protect the UI behind Authelia, a VPN, or an internal-only network.

To build against a different Authelia release:

```sh
docker build --build-arg AUTHELIA_VERSION=4.39.19 -t kubierend/authelia-webui:latest .
```

## Kubernetes

Example manifests are in `deploy/kubernetes` and use:

```yaml
image: kubierend/authelia-webui:latest
```

Mount the same writable config volume used by Authelia at `/config`, or adjust `AUTHELIA_USERS_FILE` and `AUTHELIA_CONFIG_FILE`.

## Build

```sh
npm --prefix web ci
npm --prefix web run build
go build ./cmd/server
```

The frontend build writes to `cmd/server/static`, which is embedded by the Go server. Generated assets are ignored by Git.

## Test

```sh
go test ./...
npm --prefix web run build
```

## Notes

- Authelia still needs to reload or watch its config files before changes become active.
- Generated passwords and client secrets are only shown once.
- Manual secret entry is intentionally not supported; the Authelia CLI is the single source for generated secret material.
- Presets are best-effort helpers based on Authelia’s integration docs. Check the rendered application setup notes before saving a client.
