# Build the React app
FROM node:24.21.0-alpine3.23@sha256:159fe64649038c30f8cc1ec4be3af3a6e93e3648678c31294e2c5058dbeb99f3 AS builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

# Serve with nginx
FROM nginx:1.30.5-alpine@sha256:25820c39dba41369486df729ad6697de2fab631ca809e0503e1d1e0c73d9a232
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
