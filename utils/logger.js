import chalk from 'chalk';

const levels = {
  info:  { color: chalk.cyan,    emoji: 'ℹ️ '  },
  warn:  { color: chalk.yellow,  emoji: '⚠️ '  },
  error: { color: chalk.red,     emoji: '❌ '  },
  debug: { color: chalk.magenta, emoji: '🐛 '  },
  success: { color: chalk.green, emoji: '✅'  },
  stats: { color: chalk.blue, emoji: '📢' }
};

const log = (level, ...msg) => {
  const { color, emoji } = levels[level] || levels.info;
  const timestamp = chalk.gray(new Date().toISOString());
  console.log(`${emoji} ${timestamp} ${color.bold(level.toUpperCase())} ›`, ...msg);
};

export default {
  info:    (...msg) => log('info', ...msg),
  warn:    (...msg) => log('warn', ...msg),
  error:   (...msg) => log('error', ...msg),
  debug:   (...msg) => log('debug', ...msg),
  success: (...msg) => log('success', ...msg),
  stats: (...msg) => log('stats', ...msg)
};
