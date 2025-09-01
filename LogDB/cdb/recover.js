const fs = require('fs')
const path = require('path')

const DATA = path.join(__dirname, '..', 'data')

function gatherFiles() {
  if (!fs.existsSync(DATA)) return []
  return fs.readdirSync(DATA).map(f => ({ name: f, path: path.join(DATA, f) }))
}

function tryLoadJson(p) {
  try {
    const txt = fs.readFileSync(p, 'utf8')
    const obj = JSON.parse(txt)
    if (obj && Array.isArray(obj.docs)) return obj.docs
  } catch (e) {}
  return null
}

function tryLoadBson(p) {
  try {
    const BSON = require('bson')
    const buf = fs.readFileSync(p)
    if (!buf || buf.length === 0) return null
    const arr = BSON.deserialize(buf)
    if (arr && Array.isArray(arr.docs)) return arr.docs
  } catch (e) {}
  return null
}

function recover() {
  const files = gatherFiles()
  const byCollection = {}
  for (const f of files) {
    // consider .json, .bson, .corrupt-*.bak
    const m = f.name.match(/^(?<col>[^.]+)(?:\.shard-\d+)?(?:\.(?<ext>bson|json))?(?:\.corrupt-\d+\.bak)?$/)
    if (!m || !m.groups) continue
    const col = m.groups.col
    byCollection[col] = byCollection[col] || []
    byCollection[col].push(f.path)
  }

  for (const [col, paths] of Object.entries(byCollection)) {
    console.log('Recovering', col, 'from', paths.length, 'files')
    const recovered = []
    for (const p of paths) {
      let docs = tryLoadJson(p)
      if (!docs) docs = tryLoadBson(p)
      if (docs && docs.length) {
        for (const d of docs) recovered.push(d)
      } else {
        console.warn('Could not parse', p)
      }
    }

    if (recovered.length) {
      const out = { docs: recovered }
      const outPath = path.join(DATA, col + '.recovered.json')
      fs.writeFileSync(outPath, JSON.stringify(out, null, 2), 'utf8')
      console.log('Wrote recovered file:', outPath, 'docs:', recovered.length)
    } else {
      console.log('No documents recovered for', col)
    }
  }
}

if (require.main === module) recover()

module.exports = { recover }
