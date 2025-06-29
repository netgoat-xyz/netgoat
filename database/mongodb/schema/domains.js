// cloudflareRuleEngineFull.js
import { pathToRegexp } from 'path-to-regexp'

export default class CloudflareRuleEngine {
  constructor(ruleset = []) {
    this.ruleset = ruleset // Array of { expression: string, action: string }
  }

  static getFieldValue(path, ctx) {
    const parts = path.split('.')
    let val = ctx
    for (const part of parts) {
      if (val == null) return undefined
      val = val[part]
    }
    return val
  }

  static tokenize(expr) {
    const regex = /([a-zA-Z0-9_.]+)\s+(eq|ne|lt|gt|lte|gte|contains|matches)\s+("[^"]*"|\d+(\.\d+)?)/g
    return expr.replace(regex, (_, field, op, value) => {
      const safeField = field
      const safeValue = value
      switch (op) {
        case 'eq': return `CloudflareRuleEngine.getFieldValue(\"${safeField}\", ctx) === ${safeValue}`
        case 'ne': return `CloudflareRuleEngine.getFieldValue(\"${safeField}\", ctx) !== ${safeValue}`
        case 'lt': return `CloudflareRuleEngine.getFieldValue(\"${safeField}\", ctx) < ${safeValue}`
        case 'gt': return `CloudflareRuleEngine.getFieldValue(\"${safeField}\", ctx) > ${safeValue}`
        case 'lte': return `CloudflareRuleEngine.getFieldValue(\"${safeField}\", ctx) <= ${safeValue}`
        case 'gte': return `CloudflareRuleEngine.getFieldValue(\"${safeField}\", ctx) >= ${safeValue}`
        case 'contains': return `CloudflareRuleEngine.getFieldValue(\"${safeField}\", ctx).includes(${safeValue})`
        case 'matches': return `${safeValue}.replace(/^\"|\"$/g, '') && new RegExp(${safeValue}).test(CloudflareRuleEngine.getFieldValue(\"${safeField}\", ctx))`
        default: return 'false'
      }
    })
    .replace(/\band\b/g, '&&')
    .replace(/\bor\b/g, '||')
    .replace(/\bnot\b/g, '!')
  }

  static evaluateExpression(expr, ctx) {
    try {
      const safeExpr = CloudflareRuleEngine.tokenize(expr)
      return Function('ctx', `\"use strict\"; return (${safeExpr});`)(ctx)
    } catch (err) {
      throw new Error(`Rule Syntax Error: ${err.message}`)
    }
  }

  evaluate(ctx) {
    for (const rule of this.ruleset) {
      try {
        const match = CloudflareRuleEngine.evaluateExpression(rule.expression, ctx)
        if (match) {
          return rule.action
        }
      } catch (err) {
        return { error: true, message: err.message, rule: rule.expression }
      }
    }
    return 'allow' // default action
  }
}

// Fastify plugin
export function ruleMiddleware(ruleset) {
  const engine = new CloudflareRuleEngine(ruleset)

  return async function (req, reply, next) {
    const context = {
      ip: { src: req.headers['x-forwarded-for']?.split(',')[0] || req.ip },
      http: {
        request: {
          uri: {
            path: req.url,
          },
          method: req.method,
        },
        user_agent: req.headers['user-agent'] || '',
      },
      cf: {
        bot_management: {
          score: req.headers['cf-bot-score'] ? parseFloat(req.headers['cf-bot-score']) : 50,
        }
      }
    }

    const result = engine.evaluate(context)

    if (typeof result === 'object' && result.error) {
      reply.code(400).send({ error: 'Syntax error in rule', detail: result.message, rule: result.rule })
      return
    }

    switch (result) {
      case 'block':
        reply.code(403).send('Request blocked by ruleset')
        return
      case 'log':
        console.log(`[RULE LOG] ${req.method} ${req.url} matched rule`)
        break
      case 'challenge':
        reply.code(429).send('Challenge triggered')
        return
      case 'allow':
      default:
        break
    }

    next()
  }
}
