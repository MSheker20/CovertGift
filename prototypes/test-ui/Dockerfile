# Temporary demo panel; replace with the permanent frontend when available.
FROM node:22-bookworm-slim
WORKDIR /app
COPY --chown=node:node server.mjs index.html app.js style.css ./
ENV NODE_ENV=production UI_HOST=0.0.0.0 UI_PORT=5173
USER node
EXPOSE 5173
CMD ["node", "server.mjs"]
