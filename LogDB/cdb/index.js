const { createServer } = require('./server');

const port = process.env.PORT || 3000;
createServer({ port });
