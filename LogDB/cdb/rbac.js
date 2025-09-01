// Minimal role-based access control for demo purposes
const roles = {
  admin: { can: ['*'] },
  writer: { can: ['insert', 'update', 'delete', 'read'] },
  reader: { can: ['read'] }
};

class RBAC {
  constructor() {
    // username -> { role, password? }
    this.users = new Map();
    this.users.set('admin', { role: 'admin' });
  }

  addUser(username, role = 'reader', password) {
    if (!roles[role]) throw new Error('Unknown role');
    const obj = { role };
    if (password) obj.password = password;
    this.users.set(username, obj);
  }

  getRole(username) {
    const u = this.users.get(username);
    return u && u.role;
  }

  check(username, op) {
    const u = this.users.get(username);
    if (!u) return false;
    const perms = roles[u.role].can;
    return perms.includes('*') || perms.includes(op);
  }
}

module.exports = RBAC;
