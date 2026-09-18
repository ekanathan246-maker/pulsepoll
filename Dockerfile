# Production build: one image that serves the React app via nginx and proxies
# /api to the Go backend running alongside under supervisord.
# This makes single-service deploys (Render, Fly.io, Railway) trivial.

# 1) Build the React app
FROM node:24.21.0-alpine3.23@sha256:159fe64649038c30f8cc1ec4be3af3a6e93e3648678c31294e2c5058dbeb99f3 AS fe
WORKDIR /fe
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# 2) Build the Go backend
FROM golang:1.27.1-alpine3.23@sha256:d9e2f2f07b10cc922da3e80e035c3058810b328d5aef82d2c63680967c5e2ec9 AS be
WORKDIR /be
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /server ./cmd/server

# 3) Runtime: nginx + go server under supervisord
FROM nginxinc/nginx-unprivileged:1.30.5-alpine@sha256:daa17b944bac2b578e962da4c61ad72a59233b3c63abea17113acaf4e6b9aea4
USER root
RUN apk add --no-cache supervisor
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY deploy/supervisord.conf /etc/supervisord.conf
COPY --from=fe /fe/dist /usr/share/nginx/html
COPY --from=be /server /usr/local/bin/pulsepoll
RUN chown -R 101:101 /usr/share/nginx/html /etc/nginx/conf.d /etc/supervisord.conf /usr/local/bin/pulsepoll
USER 101:101
EXPOSE 10000
CMD ["supervisord", "-c", "/etc/supervisord.conf"]
