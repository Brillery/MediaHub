FROM node:24-alpine AS stage0
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:1.25.2-alpine AS stage1
COPY --from=stage0 /app/dist/ /usr/share/nginx/html/
