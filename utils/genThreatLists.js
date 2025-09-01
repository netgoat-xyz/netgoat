const fs = require("fs");
const readline = require("readline");
const axios = require("axios");
const cliProgress = require("cli-progress");

// CONFIG
const ipinfoFile = "utils/ipinfo_lite.json";
const ddosFeedURL = "https://lists.blocklist.de/lists/all.txt";

// ---------------- FUNCTIONS ----------------
// resilient ASN parser
async function parseASNs(filePath) {
  const blockedASNs = {};
  let total = 0;
  let lineCount = 0;

  // count lines first for progress
  const rawFile = filePath.endsWith(".gz")
    ? fs.createReadStream(filePath).pipe(zlib.createGunzip())
    : fs.createReadStream(filePath);
  const lineCounter = readline.createInterface({ input: rawFile });
  for await (const _ of lineCounter) total++;

  const rl = readline.createInterface({
    input: filePath.endsWith(".gz")
      ? fs.createReadStream(filePath).pipe(zlib.createGunzip())
      : fs.createReadStream(filePath),
    crlfDelay: Infinity,
  });

  const bar = new cliProgress.SingleBar({}, cliProgress.Presets.shades_classic);
  bar.start(total, 0);

  for await (let line of rl) {
    lineCount++;
    if (!line.trim()) {
      bar.update(lineCount);
      continue;
    }

    try {
      const { asn, country_code } = JSON.parse(line);
      if (!asn || !country_code) {
        bar.update(lineCount);
        continue;
      }
      const asnNum = parseInt(asn.replace("AS", ""));
      if (!blockedASNs[country_code]) blockedASNs[country_code] = new Set();
      blockedASNs[country_code].add(asnNum);
    } catch (err) {
      // first retry attempt with trimming
      try {
        line = line.trim();
        const { asn, country_code } = JSON.parse(line);
        const asnNum = parseInt(asn.replace("AS", ""));
        if (!blockedASNs[country_code]) blockedASNs[country_code] = new Set();
        blockedASNs[country_code].add(asnNum);
      } catch (retryErr) {
        // ask user what to do
        console.log(`\n⚠️ Error parsing line ${lineCount}:`);
        console.log(line.slice(0, 200));
        console.log("Error:", retryErr.message);

        const choice = prompt("Skip this line (s) or Abort (a)? ").toLowerCase();
        if (choice === "a") {
          bar.stop();
          throw new Error("Aborted by user due to parse error.");
        }
        // else skip
      }
    }
    bar.update(lineCount);
  }

  bar.stop();

  // convert Sets to arrays
  for (const country in blockedASNs) {
    blockedASNs[country] = Array.from(blockedASNs[country]);
  }

  return blockedASNs;
}
async function fetchDDoSIPs(url) {
  console.log("Fetching DDoS IPs...");
  try {
    const response = await axios.get(url);
    const ips = response.data.split("\n").filter(ip => ip.trim());

    const bar = new cliProgress.SingleBar({
      format: 'Processing DDoS IPs |{bar}| {value}/{total} IPs',
      hideCursor: true
    }, cliProgress.Presets.shades_classic);
    bar.start(ips.length, 0);

    const finalIPs = [];
    for (let i = 0; i < ips.length; i++) {
      finalIPs.push(ips[i]);
      bar.update(i + 1);
    }
    bar.stop();

    return finalIPs;
  } catch {
    console.warn("Could not fetch DDoS feed, using example IPs");
    return ["192.0.2.1", "198.51.100.42", "203.0.113.17"];
  }
}

// ---------------- GENERATOR ----------------
async function generateThreatLists() {
  const blockedASNs = await parseASNs(ipinfoFile);
  const ddosIPs = await fetchDDoSIPs(ddosFeedURL);

  const threatLists = {
    bots: ["BadBot","curl","python-requests","SemrushBot","AhrefsBot","Sogou","Exabot","facebot","facebookexternalhit","Twitterbot","Slackbot","LinkedInBot","PinterestBot","Applebot","WhatsApp","TelegramBot","Discordbot","SkypeUriPreview"],
    aiCrawlers: ["ChatGPT","OpenAI","Bard","Claude","Perplexity","YouChat"],
    googleBots: ["Googlebot","Googlebot-Image","Googlebot-Video","Googlebot-News","Googlebot-Store","AdsBot-Google","Google-InspectionTool","Google-CloudVertexBot"],
    searchEngineCrawlers: ["Bingbot","Baiduspider","YandexBot","DuckDuckBot","Yahoo! Slurp","NaverBot","Exalead","SeznamBot","Gigabot"],
    apiClients: ["curl","python-requests","axios","PostmanRuntime","Go-http-client","Java","Wget"],
    outdatedBrowsers: ["IE6","IE7","IE8","IE9","Safari 9","Chrome 49"],
    ddosIPs,
    blockedASNs
  };

  fs.writeFileSync("utils/threatLists.js", `module.exports = ${JSON.stringify(threatLists, null, 2)};\n`);

  // ---------------- SUMMARY ----------------
  console.log("\n✅ threatLists.js generated successfully!\n");
  console.log("Summary:");
  console.log(`  Bots: ${threatLists.bots.length}`);
  console.log(`  AI Crawlers: ${threatLists.aiCrawlers.length}`);
  console.log(`  Google Bots: ${threatLists.googleBots.length}`);
  console.log(`  Search Engine Crawlers: ${threatLists.searchEngineCrawlers.length}`);
  console.log(`  API Clients: ${threatLists.apiClients.length}`);
  console.log(`  Outdated Browsers: ${threatLists.outdatedBrowsers.length}`);
  console.log(`  DDoS IPs: ${threatLists.ddosIPs.length}`);
  
  // Count total ASNs across all countries
  const totalASNs = Object.values(blockedASNs).reduce((sum, arr) => sum + arr.length, 0);
  console.log(`  Blocked ASNs: ${totalASNs}`);
}

generateThreatLists();
