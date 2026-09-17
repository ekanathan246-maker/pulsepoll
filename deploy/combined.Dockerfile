# Production build: one image that serves the React app via nginx and proxies
# /api to the Go backend running alongside under supervisord.
# This makes single-service deploys (Render, Fly.io, Railway) trivial.

# 1) Build the React app
FROM node:24-alpine AS fe
WORKDIR /fe
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# 2) Build the Go backend
FROM golang:1.27-alpine AS be
WORKDIR /be
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /server ./cmd/server

# 3) Runtime: nginx + go server under supervisord
FROM nginxinc/nginx-unprivileged:1.27-alpine
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
