const jwt = require('jsonwebtoken');
const SECRET = process.env.DB_JWT_SECRET || 'dev-secret';

class Auth {
  constructor(rbac) {
    this.rbac = rbac;
  }

  generate(user) {
    return jwt.sign({ user }, SECRET, { expiresIn: '8h' });
  }

  verify(token) {
    try {
      const p = jwt.verify(token, SECRET);
      return { ok: true, user: p.user };
    } catch (e) {
      return { ok: false, error: e.message };
    }
  }

  login(username, password) {
    // For prototype: RBAC stores username->role or username->{role,password}
    const entry = this.rbac.users.get(username);
    if (!entry) return { ok: false, error: 'no such user' };
    if (typeof entry === 'string') {
      // no password stored, allow admin only when username matches
      if (username === 'admin') return { ok: true, token: this.generate(username) };
      return { ok: false, error: 'no password' };
    }
    if (entry.password !== password) return { ok: false, error: 'invalid credentials' };
    return { ok: true, token: this.generate(username) };
  }
}

module.exports = Auth;
