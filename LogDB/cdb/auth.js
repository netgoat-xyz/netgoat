const jwt = require('jsonwebtoken');
const bcrypt = require('bcryptjs');
const speakeasy = require('speakeasy');
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

  login(username, password, mfaCode) {
    // For prototype: RBAC stores username->role or username->{role,password}
    const entry = this.rbac.users.get(username);
    if (!entry) return { ok: false, error: 'no such user' };
    if (typeof entry === 'string') {
      // no password stored, allow admin only when username matches
      if (username === 'admin') return { ok: true, token: this.generate(username) };
      return { ok: false, error: 'no password' };
    }
    if (!entry.password) return { ok: false, error: 'no password set' };
    if (!bcrypt.compareSync(password, entry.password)) {
      return { ok: false, error: 'invalid credentials' };
    }
    if (entry.mfaSecret) {
      if (!mfaCode) return { ok: false, error: 'MFA code required' };
      const verified = speakeasy.totp.verify({
        secret: entry.mfaSecret,
        encoding: 'base32',
        token: mfaCode
      });
      if (!verified) return { ok: false, error: 'Invalid MFA code' };
    }
    return { ok: true, token: this.generate(username) };
  }
}

module.exports = Auth;
