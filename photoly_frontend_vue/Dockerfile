FROM node:26-alpine AS builder

COPY . /app/
WORKDIR /app

RUN npm install -g pnpm@10.30.1 \
    && pnpm install --frozen-lockfile
# Docker serves the API through nginx on the same origin, including over LAN HTTP.
# An explicit build argument can still target a separately hosted API.
ARG VITE_APP_BASE_URL=/api
RUN VITE_APP_BASE_URL="$VITE_APP_BASE_URL" pnpm run build

FROM nginx:alpine

WORKDIR /usr/share/nginx/html

RUN rm -rf ./*

COPY --from=builder /app/dist .

COPY ./nginx.conf /etc/nginx/conf.d/default.conf

ENTRYPOINT ["nginx", "-g", "daemon off;"]
