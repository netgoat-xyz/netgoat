import fs from "fs/promises"
import path from "path"

const BASE_LOG_DIR = path.resolve(process.cwd(), "database/DomainLogs")
const domain = "google.com"
const subdomain = "@"

const LOGS_COUNT = 100000

const desktopAgents = [
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36",
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 13_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
  "Mozilla/5.0 (X11; Linux x86_64) Gecko/20100101 Firefox/128.0",
  "Mozilla/5.0 (Windows NT 10.0; Win64; rv:128.0) Gecko/20100101 Firefox/128.0",
]

const mobileAgents = [
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
  "Mozilla/5.0 (Linux; Android 14; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Mobile Safari/537.36",
  "Mozilla/5.0 (Linux; Android 13; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Mobile Safari/537.36",
  "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
]

function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)]
}

function randomIP() {
  return `${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}`
}

function makeDemoLog(date) {
  const isMobile = Math.random() > 0.5
  const userAgent = isMobile ? pick(mobileAgents) : pick(desktopAgents)

  return {
    time: date.toISOString(),
    request: pick(["GET", "POST"]),
    XForwardedFor: randomIP(),
    ReqID: `demo-${Math.random().toString(36).slice(2, 10)}`,
    path: pick(["/", "/login", "/search?q=test", "/profile", "/settings"]),
    domain,
    subdomain,
    userAgent,
    referer: pick([
      "Unknown",
      "https://google.com",
      "https://news.ycombinator.com",
      "https://reddit.com",
      "https://twitter.com"
    ]),
    remoteAddress: randomIP(),
    remotePort: Math.floor(30000 + Math.random() * 30000),
    requestHeaders: {
      host: subdomain === "@" ? domain : `${subdomain}.${domain}`,
      connection: "keep-alive",
    },
  }
}

async function generateLogs(timeframe, daysBack) {
  const logFilePath = path.join(BASE_LOG_DIR, domain, subdomain, `${timeframe}.json`)
  await fs.mkdir(path.dirname(logFilePath), { recursive: true })

  const logs = []
  const now = Date.now()
  const perDay = Math.floor(LOGS_COUNT / daysBack)

  for (let d = 0; d < daysBack; d++) {
    const dayStart = new Date(now - (d + 1) * 24 * 60 * 60 * 1000) // d days ago
    for (let i = 0; i < perDay; i++) {
      const offsetMs = Math.floor(Math.random() * 24 * 60 * 60 * 1000) // within that day
      const date = new Date(dayStart.getTime() + offsetMs)
      logs.push(makeDemoLog(date))
    }
  }

  logs.sort((a, b) => new Date(a.time) - new Date(b.time))

  await fs.writeFile(logFilePath, JSON.stringify(logs, null, 2), "utf-8")
  console.log(`✔ Generated ${logs.length} logs into ${logFilePath}`)
}

async function main() {
  await generateLogs("3mo", 90)
  await generateLogs("30d", 30)
  await generateLogs("7d", 7)
  await generateLogs("1d", 1)
}

main()
