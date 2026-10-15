FROM node:26-alpine AS builder

COPY . /app/
WORKDIR /app

RUN corepack enable && pnpm install --frozen-lockfile
RUN pnpm run build

FROM nginx:alpine

WORKDIR /usr/share/nginx/html

RUN rm -rf ./*

COPY --from=builder /app/dist .

COPY ./nginx.conf /etc/nginx/conf.d/default.conf

ENTRYPOINT ["nginx", "-g", "daemon off;"]