FROM node:24-alpine AS deps

WORKDIR /app

COPY web/package.json web/package-lock.json ./
RUN npm ci

FROM deps AS build

COPY web/ ./
RUN npm run build

FROM nginx:1.29-alpine

COPY docker/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
