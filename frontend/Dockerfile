# Kalıcı React frontend: Vite ile derlenir, ayrıcalıksız nginx ile sunulur.
# /api ve /health istekleri API_TARGET adresindeki Go API'ye iletilir (aynı origin, CORS gerekmez).
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY index.html tsconfig.json tsconfig.app.json tsconfig.node.json vite.config.ts ./
COPY public ./public
COPY src ./src
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.27-alpine
# Şablon başlangıçta API_TARGET ile doldurulur ve varsayılan siteyi değiştirir.
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /app/dist /usr/share/nginx/html
ENV API_TARGET=http://127.0.0.1:8080
EXPOSE 5173
