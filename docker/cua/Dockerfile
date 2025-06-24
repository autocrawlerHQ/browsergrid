FROM node:18-slim


WORKDIR /app

COPY docker/cua/package*.json ./

RUN npm install

COPY docker/cua/ ./

RUN npm run build

ENV NODE_ENV=production
ENV PORT=3000
ENV BROWSERMUX_URL=http://browsermux:8080

EXPOSE 3000

# Start the service
CMD ["npm", "start"] 