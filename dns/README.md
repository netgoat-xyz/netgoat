# NetGoat authoritative DNS service

This service serves active zones and DNS records from `stream-server`. It is
authoritative only: it does not recurse, forward queries, provide zone
transfers, or sign DNSSEC. It supports UDP and TCP DNS queries on the configured
DNS listener and keeps the last valid stream snapshot in memory during a stream
server outage.

The stream API exposes a filtered `/dns/zones` snapshot with a dedicated
`X-DNS-API-Key`. The DNS service polls it every five seconds by default. Records
are published from active domains and active DNS records in the `dnsrecords`
collection.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `DNS_ADDR` | `:1053` | DNS UDP and TCP listener |
| `DNS_HTTP_ADDR` | `:8053` | Health and readiness endpoints |
| `STREAM_URL` | `http://streamer:8787` | Stream server base URL |
| `STREAM_DNS_KEY` | required | Must match stream server's `DNS_READ_KEY` |
| `DNS_POLL_SECONDS` | `5` | Snapshot polling interval, 1–3600 seconds |
| `DNS_NAMESERVERS` | required | Comma-separated public NS hostnames for synthesized apex NS records |
| `DNS_SOA_MNAME` | first `DNS_NAMESERVERS` entry | SOA primary DNS server |
| `DNS_SOA_RNAME` | `hostmaster.netgoat.xyz.` | SOA contact mailbox in DNS name form |
| `DNS_SOA_TTL` | `300` | SOA TTL in seconds |
| `DNS_NEGATIVE_TTL` | `60` | SOA minimum for negative answers |
| `DNS_PROXY_IPV4` | empty | IPv4 returned for proxied A/CNAME records |
| `DNS_PROXY_IPV6` | empty | IPv6 returned for proxied AAAA/CNAME records |

The container listens on unprivileged port 1053. Publish that port as both UDP
and TCP port 53 at the host or load balancer. Configure the DNS hostnames
and their public glue addresses at the domain registrar before delegating a
zone. A single instance does not provide authoritative DNS redundancy; production
delegations should use independently reachable instances.

## Health

- `GET /healthz` reports process liveness.
- `GET /readyz` returns `503` until the first valid stream snapshot has loaded.
