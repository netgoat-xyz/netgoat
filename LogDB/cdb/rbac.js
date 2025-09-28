// Minimal role-based access control for demo purposes
const bcrypt = require('bcryptjs');
// Resource-level and action-level permissions
const roles = {
  admin: { can: [{ resource: '*', actions: ['*'] }] },
  writer: {
    can: [
      { resource: 'logs', actions: ['insert', 'update', 'delete', 'read'] },
      { resource: 'vector', actions: ['insert', 'read'] },
      { resource: 'backup', actions: ['insert'] },
      { resource: 'user', actions: ['insert'] }
    ]
  },
  reader: {
    can: [
      { resource: 'logs', actions: ['read'] },
      { resource: 'vector', actions: ['read'] }
    ]
  }
};

class RBAC {
  constructor() {
    // username -> { role, password? }
    this.users = new Map();
    this.users.set('admin', { role: 'admin' });
  }

  addUser(username, role = 'reader', password, mfaSecret) {
    if (!roles[role]) throw new Error('Unknown role');
    // Password policy: min 10 chars, at least 1 uppercase, 1 lowercase, 1 digit, 1 special char
    if (password) {
      const policy = /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[^A-Za-z\d]).{10,}$/;
      if (!policy.test(password)) {
        throw new Error('Password must be at least 10 characters and include uppercase, lowercase, digit, and special character');
      }
    }
    const obj = { role };
    if (password) {
      const salt = bcrypt.genSaltSync(10);
      obj.password = bcrypt.hashSync(password, salt);
    }
    if (mfaSecret) {
      obj.mfaSecret = mfaSecret;
    }
    this.users.set(username, obj);
  }

  getRole(username) {
    const u = this.users.get(username);
    return u && u.role;
  }

  /**
   * Checks if user has permission for a resource and action
   * Blocks access if user is currently blocked
   * @param {string} username
   * @param {string} resource
   * @param {string} action
   * @returns {boolean}
   */
  check(username, resource, action) {
    const u = this.users.get(username);
    if (!u) return false;
    if (u.blockedUntil && Date.now() < u.blockedUntil) {
      return false;
    }
    const perms = roles[u.role].can;
    for (const perm of perms) {
      if ((perm.resource === '*' || perm.resource === resource) &&
          (perm.actions.includes('*') || perm.actions.includes(action))) {
        return true;
      }
    }
    return false;
  }

  /**
   * Block a user for a given duration (ms)
   */
  blockUser(username, durationMs = 15 * 60 * 1000) {
    const u = this.users.get(username);
    if (u) {
      u.blockedUntil = Date.now() + durationMs;
      this.users.set(username, u);
    }
  }
}

module.exports = RBAC;
