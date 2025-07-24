import chalk from 'chalk';

const levels = {
  info:  { color: chalk.cyan,    emoji: 'ℹ️ '  },
  warn:  { color: chalk.yellow,  emoji: '⚠️ '  },
  error: { color: chalk.red,     emoji: '❌ '  },
  debug: { color: chalk.magenta, emoji: '🐛 '  },
  success: { color: chalk.green, emoji: '✅'  },
};

const log = (level: string, ...msg: any) => {
  const { color, emoji } = levels[level] || levels.info;
  const timestamp = chalk.gray(new Date().toISOString());
  console.log(`${emoji} ${timestamp} ${color.bold(level.toUpperCase())} ›`, ...msg);
};

export default {
  info:    (...msg: any) => log('info', ...msg),
  warn:    (...msg: any) => log('warn', ...msg),
  error:   (...msg: any) => log('error', ...msg),
  debug:   (...msg: any) => log('debug', ...msg),
  success: (...msg: any) => log('success', ...msg),
};
