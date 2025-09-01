const fetch = globalThis.fetch || require('node-fetch');

class RemoteModel {
  constructor(baseUrl, name) {
    this.base = baseUrl.replace(/\/$/, '');
    this.name = name;
  }

  async create(doc) {
    const r = await fetch(`${this.base}/odm/${this.name}/create`, { method: 'POST', body: JSON.stringify(doc), headers: {'Content-Type':'application/json'} });
    return r.json();
  }

  async find(query) {
    const r = await fetch(`${this.base}/odm/${this.name}/find`, { method: 'POST', body: JSON.stringify(query || {}), headers: {'Content-Type':'application/json'} });
    return r.json();
  }

  async findById(id) {
    const r = await fetch(`${this.base}/odm/${this.name}/${id}`);
    return r.json();
  }

  async updateById(id, patch) {
    const r = await fetch(`${this.base}/odm/${this.name}/${id}/update`, { method: 'POST', body: JSON.stringify(patch), headers: {'Content-Type':'application/json'} });
    return r.json();
  }

  async deleteById(id) {
    const r = await fetch(`${this.base}/odm/${this.name}/${id}`, { method: 'DELETE' });
    return r.json();
  }
}

module.exports = { RemoteModel };
