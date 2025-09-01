const Storage = require('./storage');

// Very small mongoose-like ODM for the prototype
class SchemaType {
  constructor(opts = {}) {
    this.type = opts.type || String;
    this.required = !!opts.required;
    this.default = opts.default;
    this.validate = opts.validate;
  }
}

class Schema {
  constructor(def) {
    this.definition = {};
    for (const k of Object.keys(def)) {
      const v = def[k];
      if (v instanceof SchemaType) this.definition[k] = v;
      else if (typeof v === 'function') this.definition[k] = new SchemaType({ type: v });
      else this.definition[k] = new SchemaType(v);
    }
  }
}

const Types = {
  String: String,
  Number: Number,
  Boolean: Boolean,
  Date: Date,
  Array: Array,
  ObjectId: String
};

class Model {
  constructor(name, schema) {
    this.name = name;
    this.schema = schema;
    this.store = new Storage(name);
  }

  _applyDefaults(doc) {
    const out = Object.assign({}, doc);
    for (const [k, spec] of Object.entries(this.schema.definition)) {
      if (out[k] === undefined && spec.default !== undefined) {
        out[k] = typeof spec.default === 'function' ? spec.default() : spec.default;
      }
    }
    return out;
  }

  _validate(doc, partial = false) {
    for (const [k, spec] of Object.entries(this.schema.definition)) {
      const val = doc[k];
      if (!partial && spec.required && (val === undefined || val === null)) {
        throw new Error(`FieldRequired:${k}`);
      }
      if (val !== undefined && val !== null) {
        const t = spec.type;
        if (t === String && typeof val !== 'string') throw new Error(`TypeMismatch:${k}:expected String`);
        if (t === Number && typeof val !== 'number') throw new Error(`TypeMismatch:${k}:expected Number`);
        if (t === Boolean && typeof val !== 'boolean') throw new Error(`TypeMismatch:${k}:expected Boolean`);
        if (t === Date && !(val instanceof Date)) {
          // allow date strings
          const d = new Date(val);
          if (isNaN(d)) throw new Error(`TypeMismatch:${k}:expected Date`);
          doc[k] = d;
        }
        if (t === Array && !Array.isArray(val)) throw new Error(`TypeMismatch:${k}:expected Array`);
        if (spec.validate && typeof spec.validate === 'function') {
          const ok = spec.validate(val);
          if (!ok) throw new Error(`ValidationFailed:${k}`);
        }
      }
    }
  }

  create(doc) {
    const withDefaults = this._applyDefaults(doc);
    this._validate(withDefaults, false);
    return this.store.insert(withDefaults);
  }

  find(query = {}) {
    return this.store.find(query);
  }

  findById(id) {
    const docs = this.store.find({ _id: id });
    return docs.length ? docs[0] : null;
  }

  updateById(id, patch) {
    // validate patch fields
    this._validate(patch, true);
    return this.store.update(id, Object.assign({}, patch, { updatedAt: new Date() }));
  }

  deleteById(id) {
    return this.store.delete(id);
  }
}

function model(name, schemaDef) {
  const schema = schemaDef instanceof Schema ? schemaDef : new Schema(schemaDef);
  return new Model(name, schema);
}

module.exports = { Schema, SchemaType, Types, model };
