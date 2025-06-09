import Fastify from 'fastify';
const app = Fastify()

app.get('/', async (request, reply) => {
    return { message: 'Welcome to the backend Server!' };
});

app.listen({ port: 3001 }, (err, address) => {
    if (err) {
        console.error(err);
        process.exit(1);
    }
    logger.info(`Backend loaded at ${address}`);
});