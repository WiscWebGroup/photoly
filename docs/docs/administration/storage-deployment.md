---
title: Storage deployment
sidebar_position: 4
---

This page explains how to allow external storage on a server. For user setup, see [Local mounts, WebDAV and SMB](../features/storage/external-storage.md).

## Allowed roots

Set the allowed roots as JSON. An empty value allows no external local roots.

```dotenv
STORAGE_LOCAL_ROOTS=[{"id":"family","name":"Family NAS","path":"/mnt/family"},{"id":"archive","name":"Archive disk","path":"/mnt/archive"}]
STORAGE_WEBDAV_ALLOWED_HOSTS=http://nextcloud,https://nas.example.internal:8443
STORAGE_WEBDAV_CA_FILE=/etc/photoly/private-ca.pem
STORAGE_SECRET_KEY=replace-with-a-long-random-secret
```

Set each separately mounted filesystem as its own allowed root.

## Docker bind mounts

Use this override:

```sh
docker compose -f docker-compose.yml -f compose.mounts.yml up -d
```

Before you start:

- Replace the example host paths with existing directories.
- Make sure UID 65532 can read, write and traverse them.

```yaml
# compose.mounts.yml
services:
  backend:
    environment:
      STORAGE_LOCAL_ROOTS: '[{"id":"family","name":"Family NAS","path":"/mnt/family"},{"id":"archive","name":"Archive disk","path":"/mnt/archive"}]'
      STORAGE_WEBDAV_ALLOWED_HOSTS: 'http://nextcloud'
    volumes:
      - type: bind
        source: /srv/nas/family
        target: /mnt/family
        bind:
          create_host_path: false
      - type: bind
        source: /srv/archive
        target: /mnt/archive
        bind:
          create_host_path: false
      # Optional private CA:
      # - /etc/photoly/private-ca.pem:/etc/photoly/private-ca.pem:ro
```

Photoly never creates or recreates a binding root. The administrator's browser sends only root IDs and relative paths.

Photoly rejects:

- Path traversal
- Symlinks
- Access that overlaps the upload, internal assets, PostgreSQL or model directories

`PHOTOLY_DATA`, `PGDATA`, `POSTGRES_DATA_DIR`, `AI_MODEL_DIR` and `MODEL_DIR` also protect custom internal locations.

## WebDAV security

- The allowlist holds **exact origins**. Include the scheme and any non-default port. Wildcards do not work.
- Public HTTP and HTTPS servers need no entry.
- Private, loopback and Docker service destinations need an explicit origin.
- Photoly checks DNS again for each connection. It pins the address for that connection.
- Photoly blocks link-local, cloud metadata and other special destinations.
- Photoly refuses redirects. Give the canonical full directory URL.
- Photoly never forwards credentials to another server.
- HTTPS verifies certificates. You can mount a private CA PEM bundle.
- There is no insecure TLS option.

## SMB security

Allow private servers by name or IP:

```yaml
services:
  backend:
    environment:
      STORAGE_SMB_ALLOWED_HOSTS: nas.example,10.0.0.10
    extra_hosts:
      - "nas.example:10.0.0.10"
```

- Names must resolve inside Docker.
- Photoly validates every DNS answer and dials a validated address.
- Photoly refuses metadata, link-local and other special addresses, even if you allowlist them.
- Photoly never sends credentials to another server through referrals.
- The pinned Go client has small guarded-handle extensions. See `photoly_server/third_party/go-smb2/PHOTOLY.md`.

## The `.photoly-system` directory

Keep `.photoly-system` reserved.

- Local transaction staging and backups live in this directory in the allowed root. It is on the same filesystem as the originals.
- WebDAV transaction backups use this directory in the bound remote root.
- Photoly excludes it from scans, ordinary operations and ZIP downloads.
- Do not remove active workspaces.

If compensation fails, Photoly keeps the backups, records a source error and requests a scan. This is compensation for ordinary failures. It is not a distributed transaction. It is not a crash recovery journal.

## API

All APIs use the authenticated `{msgCode,t}` envelope.

Admin:

- `GET /admin/storage/local-roots`
- `GET /admin/storage/local-directories?rootId=ID&relativePath=PATH`
- `GET` and `POST /admin/users/{userId}/storage/local`
- `PATCH` and `DELETE /admin/users/{userId}/storage/local/{sourceId}`

User:

- `GET` and `POST /user/storage/webdav`
- `POST /user/storage/webdav/test?sourceId=ID`. The optional ID keeps existing credentials.
- `PATCH` and `DELETE /user/storage/webdav/{sourceId}`

Status and scan:

- `GET /user/storage/sources`
- `POST /user/storage/sources/{sourceId}/scan`. Send `{ "force": true }` to verify content. Send `{nsId?, force?}` for a scoped scan. The response is `{jobId, state:"queued"}`.

Binding fields:

| Type | Fields |
| --- | --- |
| All | `name`, `enabled`, `scanInterval` |
| Local | `rootId`, `relativePath` |
| WebDAV | `url`, `username`, `password` |

Status includes:

- Source name and type
- Management owner
- Capabilities
- Revision
- Scan progress
- Last success
- Errors
- Retry count

The library checks revisions every 15 seconds. It delays the refresh during selection, dragging or editing.

For tests, see [Development and tests](../developer/development.md).
