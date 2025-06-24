export default function isMalicious(url) {
  const lower = url.toLowerCase();
  return /(\.\.\/|%2e%2e|<script|onerror|union select|or 1=1|base64,)/.test(lower);
}
