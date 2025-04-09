FROM node:18.16.0 as stage0
COPY . /app
WORKDIR /app
RUN npm install
RUN npm run build

FROM nginx:1.25.2 as statge1
COPY --from=stage0 /app/dist/ /usr/share/nginx/html/
