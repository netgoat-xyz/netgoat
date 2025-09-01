const EventEmitter = require('events')

class Ingest extends EventEmitter {
  constructor() {
    super()
    this.queue = []
    this.maxQueue = 200000 // max queued items before drop
    this.batchSize = 1000
    this.flushInterval = 500 // ms
    this.running = false
    this.processed = 0
    this.failed = 0
    this.tm = null
  }

  init(tm, opts = {}) {
    this.tm = tm
    if (opts.maxQueue) this.maxQueue = opts.maxQueue
    if (opts.batchSize) this.batchSize = opts.batchSize
    if (opts.flushInterval) this.flushInterval = opts.flushInterval
    if (!this.running) this.start()
  }

  enqueue(doc) {
    if (!doc) return { ok: false, error: 'empty' }
    if (this.queue.length >= this.maxQueue) return { ok: false, dropped: true }
    this.queue.push(doc)
    return { ok: true }
  }

  start() {
    if (this.running) return
    this.running = true
    this._timer = setInterval(() => this._flush(), this.flushInterval)
  }

  stop() {
    if (!this.running) return
    clearInterval(this._timer)
    this.running = false
  }

  async _flush() {
    if (!this.tm) return
    if (this.queue.length === 0) return
    const batch = this.queue.splice(0, this.batchSize)
    // build atomic steps
    const steps = batch.map(d => ({ store: 'requests', op: 'insert', args: [d] }))
    try {
      const res = this.tm.runAtomic(steps)
      if (res && res.ok) {
        this.processed += batch.length
        this.emit('flushed', { count: batch.length })
      } else {
        this.failed += 1
        this.emit('flushError', res)
      }
    } catch (e) {
      this.failed += 1
      this.emit('flushError', e)
    }
  }

  metrics() {
    return { queueLength: this.queue.length, processed: this.processed, failed: this.failed, batchSize: this.batchSize }
  }
}

module.exports = new Ingest()
