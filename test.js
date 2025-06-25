import { CloudflareRuleEngine } from './utils/ruleScript.js'

const ruleset = [
  {
    expression: 'ip.src eq "1.2.3.4" and http.request.uri.path contains "/admin"',
    action: 'block'
  },
  {
    expression: 'cf.bot_management.score lt 30',
    action: 'challenge'
  },
  {
    expression: 'http.user_agent contains "curl"',
    action: 'log'
  }
]

const testContexts = [
  {
    label: 'Blocked IP & admin path',
    ctx: {
      ip: { src: '1.2.3.4' },
      http: { request: { uri: { path: '/admin/dashboard' } }, user_agent: 'Mozilla/5.0' },
      cf: { bot_management: { score: 80 } }
    }
  },
  {
    label: 'Low bot score',
    ctx: {
      ip: { src: '9.9.9.9' },
      http: { request: { uri: { path: '/public' } }, user_agent: 'Googlebot' },
      cf: { bot_management: { score: 25 } }
    }
  },
  {
    label: 'User agent is curl',
    ctx: {
      ip: { src: '8.8.8.8' },
      http: { request: { uri: { path: '/' } }, user_agent: 'curl/7.79.1' },
      cf: { bot_management: { score: 99 } }
    }
  },
  {
    label: 'Nothing matches',
    ctx: {
      ip: { src: '7.7.7.7' },
      http: { request: { uri: { path: '/help' } }, user_agent: 'Mozilla/5.0' },
      cf: { bot_management: { score: 99 } }
    }
  }
]

for (const test of testContexts) {
  const engine = new CloudflareRuleEngine(ruleset)
  const result = engine.evaluate(test.ctx)
  console.log(`🧪 ${test.label}:`, result)
}
