const mongoose = require('mongoose');

const NotificationSettingsSchema = new mongoose.Schema({
  channel: { type: String, enum: ['slack', 'email', 'webhook', 'rest'], default: 'slack' },
  slackWebhook: { type: String, default: '' },
  email: { type: String, default: '' },
  webhookUrl: { type: String, default: '' },
  restUrl: { type: String, default: '' },
  template: { type: String, default: 'User {{user}} was blocked for repeated permission denials.' },
  severity: { type: String, enum: ['info', 'warning', 'critical'], default: 'warning' },
  userPrefs: [{
    userId: String,
    channel: String,
    email: String,
    slackWebhook: String,
    webhookUrl: String,
    restUrl: String,
    template: String,
    severity: String
  }],
  rolePrefs: [{
    role: String,
    channel: String,
    template: String,
    severity: String
  }],
  updatedAt: { type: Date, default: Date.now }
});

module.exports = mongoose.model('NotificationSettings', NotificationSettingsSchema);
