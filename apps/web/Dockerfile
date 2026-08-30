FROM node:22-alpine

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
ARG GATEWAY_URL
ENV GATEWAY_URL=$GATEWAY_URL
RUN npm run build

CMD ["npm", "start"]
