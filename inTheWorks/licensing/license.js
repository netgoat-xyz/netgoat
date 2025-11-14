import { generateKeyPairSync, sign, verify, createHash } from "crypto";
import { deflateSync, inflateSync } from "zlib";

const B32 = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
function base32Encode(buf) {
  let bits = 0,
    val = 0,
    out = "";
  for (let b of buf) {
    val = (val << 8) | b;
    bits += 8;
    while (bits >= 5) {
      out += B32[(val >>> (bits - 5)) & 31];
      bits -= 5;
    }
  }
  if (bits > 0) out += B32[(val << (5 - bits)) & 31];
  return out;
}
function base32Decode(s) {
  let out = [],
    bits = 0,
    val = 0;
  for (let ch of s) {
    const idx = B32.indexOf(ch);
    if (idx === -1) throw new Error("bad base32");
    val = (val << 5) | idx;
    bits += 5;
    if (bits >= 8) {
      out.push((val >>> (bits - 8)) & 0xff);
      bits -= 8;
    }
  }
  return Buffer.from(out);
}
function hexToKeyObj(hex, type, fmt = "der") {
  return { key: Buffer.from(hex, "hex"), type, format: fmt };
}
function group24(s) {
  return s.match(/.{1,24}/g).join("-");
}

export function makeKeypair() {
  const { publicKey, privateKey } = generateKeyPairSync("ed25519");
  return {
    pub: publicKey.export({ type: "spki", format: "der" }).toString("hex"),
    priv: privateKey.export({ type: "pkcs8", format: "der" }).toString("hex"),
  };
}

export function createCompactLicense({
  product = "P",
  version = "1",
  meta = {},
  privHex,
  kid = "k1",
}) {
  const payload = {
    iat: Math.floor(Date.now() / 1000),
    meta,
    id: meta.id || null,
    exp: meta.exp || null,
    kid,
  };
  const payloadBuf = Buffer.from(JSON.stringify(payload));
  const comp = deflateSync(payloadBuf);
  const sig = sign(null, comp, hexToKeyObj(privHex, "pkcs8"));
  const sep = Buffer.from([0]);
  const blob = Buffer.concat([comp, sep, sig]);
  const b32 = base32Encode(blob);
  const groups = group24(b32);
  const digest = createHash("sha256").update(blob).digest().slice(0, 5);
  const recovery = base32Encode(digest).slice(0, 8);
  return `${product}-${version}.${groups}:${recovery}`;
}

export function verifyCompactLicense(compact, getPubHexForKid) {
  try {
    const [prodver, rest] = compact.split(".");
    if (!rest) return { ok: false, reason: "bad_format" };
    const [groupsPart, recovery] = rest.split(":");
    if (!recovery) return { ok: false, reason: "missing_recovery" };
    const b32 = groupsPart.replace(/-/g, "");
    const blob = base32Decode(b32);
    const expectedRec = base32Encode(
      createHash("sha256").update(blob).digest().slice(0, 5)
    ).slice(0, 8);
    if (expectedRec !== recovery)
      return { ok: false, reason: "recovery_mismatch" };
    const zIdx = blob.indexOf(0);
    if (zIdx <= 0) return { ok: false, reason: "bad_blob" };
    const comp = blob.slice(0, zIdx);
    const sig = blob.slice(zIdx + 1);
    const payloadBuf = inflateSync(comp);
    const payload = JSON.parse(payloadBuf.toString());
    const pubHex = getPubHexForKid(payload.kid);
    if (!pubHex) return { ok: false, reason: "unknown_kid" };
    const ok = verify(null, comp, hexToKeyObj(pubHex, "spki"), sig);
    if (!ok) return { ok: false, reason: "bad_sig", payload };
    const now = Math.floor(Date.now() / 1000);
    if (payload.exp && now > payload.exp)
      return { ok: false, reason: "expired", payload };
    return { ok: true, payload, product: prodver };
  } catch (e) {
    return { ok: false, reason: "exception", error: e.message };
  }
}
