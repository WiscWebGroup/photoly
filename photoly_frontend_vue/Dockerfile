FROM node:21-alpine3.19 AS builder

COPY . /app/
WORKDIR /app

RUN npm install
RUN npm install --save leaflet
RUN npm run build

FROM nginx:alpine

WORKDIR /usr/share/nginx/html

RUN rm -rf ./*

COPY --from=builder /app/dist .

COPY ./nginx.conf /etc/nginx/conf.d/default.conf

ENTRYPOINT ["nginx", "-g", "daemon off;"]