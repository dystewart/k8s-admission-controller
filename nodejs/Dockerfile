FROM node:21-alpine

# App directory
WORKDIR /usr/src/app

# Install app dependencies
COPY package*.json ./

RUN npm ci --only=production

# Bundle app source
COPY . .

CMD ["node", "server.js"]
