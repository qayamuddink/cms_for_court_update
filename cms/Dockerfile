FROM node:20-alpine

WORKDIR /app

COPY package.json ./
COPY server.js ./
COPY src ./src
COPY public ./public

EXPOSE 3000

HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:3000/healthz || exit 1

CMD ["node", "server.js"]
