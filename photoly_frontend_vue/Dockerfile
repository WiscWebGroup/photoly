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

FROM nginx:alpine

WORKDIR /usr/share/nginx/html

RUN rm -rf ./*

COPY --from=builder /app/dist .
COPY --from=builder /app/dist/legal /usr/share/photoly/licenses/frontend
# Installed Alpine package versions, so the corresponding upstream sources can be identified.
RUN apk info -v | sort > /usr/share/photoly/licenses/system-packages.txt

# The stock nginx entrypoint renders templates/*.template (envsubst) into conf.d at start.
# BACKEND_UPSTREAM is host:port of the Photoly backend; override it when the service name
# `backend` is ambiguous (e.g. the web container also joins a shared proxy network).
ENV BACKEND_UPSTREAM=backend:8084
COPY ./nginx.conf /etc/nginx/templates/default.conf.template
