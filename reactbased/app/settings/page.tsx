
import React, { useState, useEffect } from "react";

const defaultConfig = {
  channel: "slack",
  slackWebhook: "",
  email: "",
  webhookUrl: "",
  restUrl: ""
};

export default function SettingsPage() {
  const [config, setConfig] = useState(defaultConfig);
  const [loading, setLoading] = useState(true);
  const [status, setStatus] = useState("");

  useEffect(() => {
    fetch("/api/settings")
      .then((r) => r.json())
      .then((data) => {
        setConfig({ ...defaultConfig, ...data });
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  function handleChange(e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) {
    setConfig({ ...config, [e.target.name]: e.target.value });
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setStatus("Saving...");
    const res = await fetch("/api/settings", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(config)
    });
    if (res.ok) {
      setStatus("Settings saved!");
    } else {
      setStatus("Failed to save settings.");
    }
  }

  return (
    <div className="max-w-xl mx-auto p-8">
      <h1 className="text-2xl font-bold mb-4">Notification Settings</h1>
      {loading ? (
        <div>Loading...</div>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-4">
          <label className="block">
            <span className="font-semibold">Channel</span>
            <select name="channel" value={config.channel} onChange={handleChange} className="block w-full mt-1">
              <option value="slack">Slack</option>
              <option value="email">Email</option>
              <option value="webhook">Webhook</option>
              <option value="rest">REST API</option>
            </select>
          </label>
          {config.channel === "slack" && (
            <label className="block">
              <span className="font-semibold">Slack Webhook URL</span>
              <input type="text" name="slackWebhook" value={config.slackWebhook} onChange={handleChange} className="block w-full mt-1" />
            </label>
          )}
          {config.channel === "email" && (
            <label className="block">
              <span className="font-semibold">Email Address</span>
              <input type="email" name="email" value={config.email} onChange={handleChange} className="block w-full mt-1" />
            </label>
          )}
          {config.channel === "webhook" && (
            <label className="block">
              <span className="font-semibold">Webhook URL</span>
              <input type="text" name="webhookUrl" value={config.webhookUrl} onChange={handleChange} className="block w-full mt-1" />
            </label>
          )}
          {config.channel === "rest" && (
            <label className="block">
              <span className="font-semibold">REST API URL</span>
              <input type="text" name="restUrl" value={config.restUrl} onChange={handleChange} className="block w-full mt-1" />
            </label>
          )}
          <button type="submit" className="bg-blue-600 text-white px-4 py-2 rounded">Save Settings</button>
          {status && <div className="mt-2 text-sm">{status}</div>}
        </form>
      )}
    </div>
  );
}
