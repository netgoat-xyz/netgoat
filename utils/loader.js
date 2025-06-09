import { readdir } from 'fs/promises';
import { basename, extname } from 'path';
const dir = new URL('.', import.meta.url);

const files = await readdir(dir);

for (const file of files) {
  if (file === 'loader.js' || !file.endsWith('.js')) continue;

  const name = basename(file, extname(file));
  const mod = await import(`./${file}`);
  globalThis[name] = mod.default || mod;
}
