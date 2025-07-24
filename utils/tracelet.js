export default function tracelet(region = 'hell') {
  const timestamp = Date.now().toString(16)
  const random = Math.random().toString(16).slice(2, 8)
  return `hx${timestamp}-${random}-${region}`
}
