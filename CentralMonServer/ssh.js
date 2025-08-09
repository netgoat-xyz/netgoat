const { Server } = require('ssh2');
const repl = require('repl');
const fs = require('fs');
const os = require('os');

const username = 'ducky';
const hostname = os.hostname();

const hostKey = fs.readFileSync('host_ed25519');

const server = new Server({ hostKeys: [hostKey] }, (client) => {
  console.log('> Client connected');

  client.on('authentication', (ctx) => {
    if (ctx.method === 'password' && ctx.username === username && ctx.password === 'quack') {
      ctx.accept();
    } else {
      ctx.reject();
    }
  });

  client.on('ready', () => {
    console.log('> Authenticated');

    client.on('session', (accept) => {
      const session = accept();

      session.on('pty', (accept, _, info) => {
        accept && accept();
      });

      session.on('shell', (accept) => {
        const stream = accept();
        const bashPrompt = `\x1b[32mducky@NetGoat\x1b[0m \x1b[34m$\x1b[0m `
        const r = repl.start({
          input: stream,
          output: stream,
          prompt: bashPrompt,
          terminal: true,
          useGlobal: false,
eval: (cmd, context, filename, callback) => {
  cmd = cmd.trim();

  // ⛔ If empty, return nothing
  if (cmd === '') return callback(null);

  // Bash-style: call functions without ()
  if (context[cmd] && typeof context[cmd] === 'function') {
    try {
      const result = context[cmd]();
      callback(null, result);
    } catch (err) {
      callback(err);
    }
  } else {
            try {
              const result = eval(`with (ctx) { ${cmd} }`);
              callback(null, result);
            } catch (err) {
              callback(null, `${err.name}: ${err.message}`);
            }
  }
}
        });

        r.context.sayHello = () => '🐣 quack from fake bash shell';
        r.context.whoami = () => username;
        r.context.hostname = () => hostname;
        r.context.clear = () => ''; // emulate shell `clear`
        
        r.on('exit', () => {
          stream.end();
        });
      });
    });
  });

  client.on('end', () => console.log('> Client disconnected'));
});

server.listen(2222, '0.0.0.0', () => {
  console.log('> Fake BASH SSH server up on port 2222');
});
