// Notification system for RBAC alerts
// Supports Slack, Email, Webhook, REST API (configurable)

const fetch = globalThis.fetch || require('node-fetch');
const mongoose = require('mongoose');
let NotificationSettings;
try {
  NotificationSettings = require('../../database/mongodb/schema/notificationSettings');
} catch (e) {}

class Notifier {
  constructor(config) {
    this.config = config || {};
  }

  async loadConfig() {
    try {
      if (!NotificationSettings) return {};
      if (mongoose.connection.readyState === 0) {
        await mongoose.connect(process.env.MONGODB_URI || 'mongodb://localhost:27017/netgoat');
      }
      const doc = await NotificationSettings.findOne();
      return doc ? doc.toObject() : {};
    } catch (e) { return {}; }
  }

  // Find user/role preferences and merge with global config
  getPrefs(config, meta) {
    let prefs = { ...config };
    if (meta && meta.userId && config.userPrefs) {
      const userPref = config.userPrefs.find(u => u.userId === meta.userId);
      if (userPref) prefs = { ...prefs, ...userPref };
    }
    if (meta && meta.role && config.rolePrefs) {
      const rolePref = config.rolePrefs.find(r => r.role === meta.role);
      if (rolePref) prefs = { ...prefs, ...rolePref };
    }
    return prefs;
  }

  // Render template with meta
  renderTemplate(template, meta) {
    return template.replace(/{{(\w+)}}/g, (_, k) => meta[k] || '');
  }

  async send({ channel, subject, message, meta, severity }) {
    const config = await this.loadConfig();
    const prefs = this.getPrefs(config, meta);
    const tpl = prefs.template || 'User {{user}} was blocked for repeated permission denials.';
    const renderedMsg = this.renderTemplate(tpl, meta);
    const sev = severity || prefs.severity || 'warning';
    switch (channel || prefs.channel || prefs.default) {
      case 'slack':
        return this.sendSlack(renderedMsg, meta, prefs, sev);
      case 'email':
        return this.sendEmail(subject, renderedMsg, meta, prefs, sev);
      case 'webhook':
        return this.sendWebhook(renderedMsg, meta, prefs, sev);
      case 'rest':
        return this.sendRest(renderedMsg, meta, prefs, sev);
      case 'pagerduty':
        return this.sendPagerDuty(renderedMsg, meta, prefs, sev);
      case 'opsgenie':
        return this.sendOpsgenie(renderedMsg, meta, prefs, sev);
      default:
        return Promise.resolve(false);
    }
  }

  async sendSlack(message, meta, config, severity) {
    if (!config.slackWebhook) return false;
    return fetch(config.slackWebhook, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text: `[${severity.toUpperCase()}] ${message}`, meta })
    });
  }

  async sendEmail(subject, message, meta, config, severity) {
    // Placeholder: integrate nodemailer or similar
    // e.g., nodemailer.sendMail({ to: config.email, subject: `[${severity}] ${subject}`, text: message })
    return false;
  }

  async sendWebhook(message, meta, config, severity) {
    if (!config.webhookUrl) return false;
    return fetch(config.webhookUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: `[${severity.toUpperCase()}] ${message}`, meta })
    });
  }

  async sendRest(message, meta, config, severity) {
    if (!config.restUrl) return false;
    return fetch(config.restUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: `[${severity.toUpperCase()}] ${message}`, meta })
    });
  }

  async sendPagerDuty(message, meta, config, severity) {
    // TODO: Integrate PagerDuty API
    return false;
  }

  async sendOpsgenie(message, meta, config, severity) {
    // TODO: Integrate Opsgenie API
    return false;
  }
}

module.exports = Notifier;
