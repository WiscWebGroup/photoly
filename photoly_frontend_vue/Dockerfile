FROM node:26-alpine AS builder

COPY . /app/
WORKDIR /app

RUN npm install -g pnpm@10.30.1 \
    && pnpm install --frozen-lockfile
# Docker serves the API through nginx on the same origin, including over LAN HTTP.
# An explicit build argument can still target a separately hosted API.
ARG VITE_APP_BASE_URL=/api
ARG PHOTOLY_REVISION=development
ARG PHOTOLY_SOURCE_URL=
ENV VITE_PHOTOLY_REVISION=$PHOTOLY_REVISION VITE_PHOTOLY_SOURCE_URL=$PHOTOLY_SOURCE_URL
RUN VITE_APP_BASE_URL="$VITE_APP_BASE_URL" pnpm run build

FROM nginx:1.31.8@sha256:165ddde8751e75a1c907d24ef10d9f6deb08b57e0fe56caf67512486aff76d82

# Refresh installed Debian packages so their exact sources remain in the signed indexes.
RUN apt-get update && apt-get upgrade -y && rm -rf /var/lib/apt/lists/*

WORKDIR /usr/share/nginx/html

RUN rm -rf ./*

COPY --from=builder /app/dist .
COPY --from=builder /app/dist/legal /usr/share/photoly/licenses/frontend
COPY scripts/container-notices.sh /tmp/container-notices.sh
RUN sh /tmp/container-notices.sh && rm /tmp/container-notices.sh

# The stock nginx entrypoint renders templates/*.template (envsubst) into conf.d at start.
# BACKEND_UPSTREAM is host:port of the Photoly backend; override it when the service name
# `backend` is ambiguous (e.g. the web container also joins a shared proxy network).
ENV BACKEND_UPSTREAM=backend:8084
COPY ./nginx.conf /etc/nginx/templates/default.conf.template
