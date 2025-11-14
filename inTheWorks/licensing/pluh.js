import { makeKeypair, createCompactLicense, verifyCompactLicense } from "./license.js"

const kp = makeKeypair()
const pubHex = kp.pub
const privHex = kp.priv
const lic = createCompactLicense({
  product:"NETG",
  version:"2",
  meta:{ id:"user-42", features:{ pro:true, seats:3 }, exp: Math.floor(Date.now()/1000)+30*24*3600 },
  privHex,
  kid:"k1"
})
function getPub(k){ return k==="k1" ? pubHex : null }
console.log(lic)
console.log(verifyCompactLicense(lic, getPub))
