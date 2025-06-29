import Fastify from 'fastify';
const app = Fastify()

app.get('/', async (request, reply) => {
    return { message: 'Welcome to the backend Server!' };
});

app.get('/monitor.js', async (request, reply) => {
    reply.type('application/javascript');
    return `
      (() => {
        const sessionId = localStorage._sessionId ||= crypto.randomUUID();
        const events = [];

        rrweb.record({
          emit(event) {
            events.push(event);
          },
          recordCanvas: true
        });

        window.addEventListener('error', e => {
          events.push({
            type: 'custom',
            timestamp: Date.now(),
            data: {
              tag: 'error',
              message: e.message,
              stack: e.error?.stack || null
            }
          });
        });

        setInterval(() => {
          if (!events.length) return;

          const payload = {
            sessionId,
            timestamp: Date.now(),
            events: events.splice(0, events.length)
          };

          navigator.sendBeacon('/__monitor/replay', JSON.stringify(payload));
        }, 3000);
      })();
    `;
});

app.listen({ port: 3001 }, (err, address) => {
    if (err) {
        console.error(err);
        process.exit(1);
    }
    logger.info(`Backend loaded at ${address}`);
});