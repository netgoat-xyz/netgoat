# MongoDB Data Synchronization for NetGoat Agent

This document describes the MongoDB collections and data structures that the NetGoat Go agent should watch using Change Streams for real-time configuration updates.

## Architecture Overview

The frontend (Next.js) writes configuration directly to MongoDB. The external API server watches MongoDB Change Streams and broadcasts changes to connected agents via WebSocket/SSE. Each agent maintains its own in-memory configuration cache.

```
Frontend (Next.js) → MongoDB ← External API Server → Go Agents
```

## Collections to Watch

### 1. `domains` Collection

**Purpose**: Main domain/subdomain configuration with WAF rules and routing settings.

**Key Fields:**
- `_id`: ObjectId - Unique identifier
- `user_id`: ObjectId - Legacy user reference (deprecated)
- `team_id`: ObjectId - Team that owns this domain **[REQUIRED]**
- `domain`: String - Primary domain (e.g., "example.com")
- `target_url`: String - Backend URL to proxy to
- `auto_ssl`: Boolean - Enable/disable automatic SSL
- `ssl_enabled`: Boolean - SSL configuration status
- `active`: Boolean - Whether domain is currently active
- `verified`: Boolean - Domain ownership verification status

**Subdomains Array** (`subdomains`):
```javascript
{
  subdomain: String,        // e.g., "api"
  full_domain: String,      // e.g., "api.example.com"
  target_url: String,       // Backend URL for this subdomain
  ssl_enabled: Boolean,
  active: Boolean,
  waf_rules: [            // WAF rules specific to subdomain
    {
      name: String,
      expression: String,
      action: "BLOCK" | "ALLOW" | "LOG",
      priority: Number,
      enabled: Boolean
    }
  ],
  total_requests: Number,
  total_blocked: Number
}
```

**Top-Level WAF Rules** (`waf_rules`):
- Apply to main domain and all subdomains
- Same structure as subdomain WAF rules

**Settings** (`settings`):
```javascript
{
  rate_limit: Number,           // Requests per second
  cache_enabled: Boolean,
  cache_ttl: Number,           // Seconds
  compression_enabled: Boolean,
  http_to_https_redirect: Boolean,
  hsts_enabled: Boolean,
  cors_enabled: Boolean,
  cors_origins: [String]
}
```

**Change Stream Actions:**
- `insert`: New domain added → Load configuration
- `update`: Domain/subdomain/WAF modified → Reload affected routes
- `delete`: Domain removed → Remove routes and cleanup

---

### 2. `teams` Collection

**Purpose**: Team memberships, quotas, and aggregate statistics.

**Key Fields:**
- `_id`: ObjectId
- `slug`: String - URL-friendly identifier
- `name`: String
- `members`: Array of team members
  ```javascript
  {
    user_id: ObjectId,
    role: "owner" | "admin" | "member" | "viewer",
    joined_at: Date
  }
  ```
- `max_domains`: Number - Domain quota limit
- `domain_count`: Number - Current domains count
- `total_requests`: Number - Aggregate request counter
- `total_blocked`: Number - Aggregate blocked requests
- `total_bandwidth`: Number - Aggregate bytes transferred
- `plan`: "free" | "pro" | "enterprise"
- `active`: Boolean

**Change Stream Actions:**
- `update`: Stats changes → Update metrics dashboards
- Agent doesn't need to actively watch this unless implementing team-level rate limiting

---

### 3. `dnsrecords` Collection

**Purpose**: DNS records management for domains.

**Key Fields:**
- `_id`: ObjectId
- `team_id`: ObjectId **[REQUIRED]**
- `domain_id`: ObjectId - References `domains` collection
- `domain`: String - Full domain name
- `type`: "A" | "AAAA" | "CNAME" | "MX" | "TXT" | "NS" | "SRV" | "CAA"
- `name`: String - Record name/subdomain (@ for root)
- `value`: String - Record value (IP, hostname, text)
- `ttl`: Number - Time to live in seconds
- `priority`: Number - For MX/SRV records
- `proxied`: Boolean - Whether traffic goes through NetGoat proxy
- `active`: Boolean
- `propagated`: Boolean - DNS propagation status

**Change Stream Actions:**
- `insert`: New DNS record → Update agent's DNS resolver/cache
- `update`: DNS record modified → Flush cache, reload record
- `delete`: DNS record removed → Remove from cache

**Agent Behavior:**
- If `proxied: true` → Route through NetGoat (apply WAF, stats, etc.)
- If `proxied: false` → Return actual DNS record without proxying

---

### 4. `proxyconfigs` Collection

**Purpose**: Reverse proxy, load balancing, and upstream server configuration.

**Key Fields:**
- `_id`: ObjectId
- `team_id`: ObjectId **[REQUIRED]**
- `domain_id`: ObjectId - References `domains` collection
- `name`: String - Configuration identifier
- `subdomain`: String | null - null for domain-level config

**Upstream Servers** (`upstream_servers`):
```javascript
[
  {
    url: String,                    // e.g., "https://backend1.internal:8080"
    weight: Number,                 // Load balancing weight
    max_fails: Number,              // Consecutive failures before marking down
    fail_timeout: Number,           // Seconds to wait before retry
    backup: Boolean,                // Use only when primary servers are down
    down: Boolean,                  // Manual down status
    health_status: "healthy" | "unhealthy" | "unknown",
    last_health_check: Date
  }
]
```

**Load Balancing** (`load_balancing`):
- `"round_robin"`: Distribute evenly
- `"least_connections"`: Route to server with fewest active connections
- `"ip_hash"`: Sticky sessions based on client IP
- `"weighted"`: Respect server weights

**Health Check** (`health_check`):
```javascript
{
  enabled: Boolean,
  interval: Number,         // Seconds between checks
  timeout: Number,          // Request timeout
  path: String,             // Health check endpoint (e.g., "/health")
  expected_status: Number,  // Expected HTTP status code
  fall: Number,             // Consecutive failures to mark unhealthy
  rise: Number              // Consecutive successes to mark healthy
}
```

**Timeouts** (all in seconds):
- `connect_timeout`
- `send_timeout`
- `read_timeout`
- `keepalive_timeout`

**Additional Config:**
- `custom_headers`: Array of `{name, value}` to inject in requests
- `path_rewrites`: Array of `{from, to, regex}` for URL rewriting
- `preserve_host`: Boolean - Keep original Host header
- `websocket_enabled`: Boolean - WebSocket proxy support
- `ssl_verify`: Boolean - Verify upstream SSL certificates
- `enabled`: Boolean - Whether this config is active

**Change Stream Actions:**
- `insert`: New proxy config → Initialize upstream servers, start health checks
- `update`: Config modified → Reload upstream servers, restart health checks
- `delete`: Config removed → Stop health checks, remove upstream pool

**Agent Behavior:**
- Maintain connection pools per upstream server
- Run background health checks at configured intervals
- Track `total_requests`, `total_errors`, `avg_response_time` and periodically write back to MongoDB
- Respect `max_fails` and `fail_timeout` for automatic failover

---

## Change Stream Implementation

### Filter Pattern

Watch only documents belonging to specific teams or all changes:

```javascript
pipeline = [
  {
    $match: {
      $or: [
        { "fullDocument.team_id": { $exists: true } },
        { "updateDescription.updatedFields.team_id": { $exists: true } }
      ]
    }
  }
]
```

### Change Stream Events

**Event Types:**
- `insert`: New document created
- `update`: Document modified (check `updateDescription.updatedFields`)
- `delete`: Document removed (`documentKey._id`)
- `replace`: Entire document replaced (rare)

**Example Change Event (Domain Update):**
```javascript
{
  operationType: "update",
  clusterTime: Timestamp,
  fullDocument: {
    _id: ObjectId("..."),
    team_id: ObjectId("..."),
    domain: "example.com",
    target_url: "https://backend.example.internal",
    active: true,
    waf_rules: [...],
    // ... full document
  },
  documentKey: { _id: ObjectId("...") },
  updateDescription: {
    updatedFields: {
      "settings.rate_limit": 100,
      "waf_rules.0.enabled": false
    },
    removedFields: []
  }
}
```

---

## Agent Configuration Reload Strategy

### 1. **Startup**: Full Configuration Load
```go
// On agent startup
domains := loadAllActiveDomains()
dnsRecords := loadAllDNSRecords()
proxyConfigs := loadAllProxyConfigs()

// Build in-memory routing table
for domain := range domains {
    registerRoutes(domain)
    loadSubdomainRoutes(domain)
    loadDomainWAF(domain)
}
```

### 2. **Change Stream**: Incremental Updates
```go
// Listen to MongoDB change streams
stream := watchCollections(["domains", "dnsrecords", "proxyconfigs"])

for event := range stream {
    switch event.OperationType {
    case "insert":
        handleInsert(event.FullDocument)
    case "update":
        handleUpdate(event.DocumentKey, event.UpdateDescription)
    case "delete":
        handleDelete(event.DocumentKey)
    }
}
```

### 3. **Graceful Reload**:
- Use read-write locks for routing table access
- Atomic swaps for configuration updates
- Drain existing connections before removing routes (optional)

---

## Data Consistency

### Team-Based Isolation
Every major document has a `team_id`. Agents can optionally:
- Filter change streams by team (for multi-tenant deployments)
- Implement team-level quotas and rate limits
- Segregate logs and metrics by team

### Eventual Consistency
- Changes propagate within 1-2 seconds via Change Streams
- Brief inconsistency window is acceptable (CAP theorem: AP system)
- No distributed locking needed - MongoDB provides ordering guarantees within a replica set

---

## Statistics Write-Back

The agent should periodically update request counters and metrics back to MongoDB:

```go
// Every 10-30 seconds
func syncStatistics() {
    updates := collectDomainStats()
    
    for domainId, stats := range updates {
        db.domains.updateOne(
            { _id: domainId },
            { $inc: {
                total_requests: stats.RequestsDelta,
                total_blocked: stats.BlockedDelta
            }},
            { $set: {
                last_request_at: time.Now()
            }}
        )
    }
    
    // Similarly for ProxyConfig stats
    db.proxyconfigs.updateMany(...)
}
```

**Aggregation to Teams:**
The backend aggregates domain stats to team totals periodically (not agent responsibility).

---

## WAF Expression Language

**Available Properties:**
- `request.path` - URL path
- `request.method` - HTTP method (GET, POST, etc.)
- `request.ip` - Client IP address
- `request.country` - GeoIP country code
- `request.user_agent` - User-Agent header
- `request.rate` - Requests per second from this IP

**Functions:**
- `contains(string, substring)` - Case-insensitive substring match
- `matches(string, regex)` - Regex pattern match
- `startsWith(string, prefix)`
- `endsWith(string, suffix)`

**Example Rules:**
```javascript
// SQL Injection
"contains(request.path, 'SELECT') || contains(request.path, 'UNION')"

// Geographic block
"request.country == 'CN' || request.country == 'RU'"

// Rate limiting
"request.rate > 100"
```

**Action Types:**
- `BLOCK`: Return 403 Forbidden, increment blocked counter
- `ALLOW`: Explicitly allow (bypass lower priority rules)
- `LOG`: Allow but log for analysis

**Evaluation:**
- Rules evaluated by priority (highest first)
- First matching rule applies (short-circuit evaluation)
- Domain-level rules apply to all subdomains unless subdomain overrides

---

## Security Considerations

1. **Authentication**: Agent connects to MongoDB with dedicated service account (read-only on most collections, write to stats fields)
2. **TLS**: All MongoDB connections use TLS
3. **Network**: MongoDB replica set accessible only from agent cluster (private network)
4. **Validation**: Agent must validate all configuration before applying (malformed WAF expressions, invalid URLs, etc.)

---

## Monitoring & Observability

**Metrics to Track:**
- Change stream lag (time between MongoDB write and agent reload)
- Configuration reload duration
- Number of active domains/DNS records/proxy configs loaded
- Health check results per upstream server
- WAF rule match rate per rule

**Logging:**
- Log all configuration changes (INFO level)
- Log WAF blocks with rule name (WARN level)
- Log upstream server failures (ERROR level)

---

## Example: Complete Domain Configuration

```javascript
// domains collection
{
  _id: ObjectId("650a1b2c3d4e5f6789012345"),
  team_id: ObjectId("650a1b2c3d4e5f6789abcdef"),
  domain: "example.com",
  target_url: "https://backend.internal:8080",
  auto_ssl: true,
  ssl_enabled: true,
  active: true,
  verified: true,
  
  subdomains: [
    {
      subdomain: "api",
      full_domain: "api.example.com",
      target_url: "https://api-backend.internal:8081",
      ssl_enabled: true,
      active: true,
      waf_rules: [
        {
          name: "api-rate-limit",
          expression: "request.rate > 1000",
          action: "BLOCK",
          priority: 10,
          enabled: true
        }
      ],
      total_requests: 150234,
      total_blocked: 523
    }
  ],
  
  waf_rules: [
    {
      name: "sql-injection",
      expression: "contains(request.path, 'SELECT') || contains(request.path, 'UNION')",
      action: "BLOCK",
      priority: 9,
      enabled: true
    },
    {
      name: "xss-protection",
      expression: "contains(request.path, '<script>')",
      action: "BLOCK",
      priority: 8,
      enabled: true
    }
  ],
  
  settings: {
    rate_limit: 100,
    cache_enabled: true,
    cache_ttl: 300,
    compression_enabled: true,
    http_to_https_redirect: true,
    hsts_enabled: true,
    cors_enabled: true,
    cors_origins: ["https://app.example.com"]
  },
  
  total_requests: 1234567,
  total_blocked: 3456,
  total_bandwidth: 9876543210,
  last_request_at: ISODate("2024-02-12T10:30:00Z")
}
```

```javascript
// proxyconfigs collection
{
  _id: ObjectId("650a1b2c3d4e5f6789012346"),
  team_id: ObjectId("650a1b2c3d4e5f6789abcdef"),
  domain_id: ObjectId("650a1b2c3d4e5f6789012345"),
  name: "main-backend-pool",
  subdomain: null,
  
  upstream_servers: [
    {
      url: "https://backend1.internal:8080",
      weight: 2,
      max_fails: 3,
      fail_timeout: 30,
      backup: false,
      down: false,
      health_status: "healthy",
      last_health_check: ISODate("2024-02-12T10:35:00Z")
    },
    {
      url: "https://backend2.internal:8080",
      weight: 1,
      max_fails: 3,
      fail_timeout: 30,
      backup: false,
      down: false,
      health_status: "healthy",
      last_health_check: ISODate("2024-02-12T10:35:00Z")
    }
  ],
  
  load_balancing: "weighted",
  
  health_check: {
    enabled: true,
    interval: 30,
    timeout: 5,
    path: "/health",
    expected_status: 200,
    fall: 3,
    rise: 2
  },
  
  connect_timeout: 60,
  send_timeout: 60,
  read_timeout: 60,
  keepalive_timeout: 75,
  
  custom_headers: [
    { name: "X-Backend-Pool", value: "main" },
    { name: "X-Request-ID", value: "$request_id" }
  ],
  
  path_rewrites: [
    { from: "/api/v1", to: "/v2", regex: false }
  ],
  
  preserve_host: true,
  websocket_enabled: true,
  ssl_verify: true,
  enabled: true,
  
  total_requests: 500000,
  total_errors: 234,
  avg_response_time: 45.6
}
```

```javascript
// dnsrecords collection
{
  _id: ObjectId("650a1b2c3d4e5f6789012347"),
  team_id: ObjectId("650a1b2c3d4e5f6789abcdef"),
  domain_id: ObjectId("650a1b2c3d4e5f6789012345"),
  domain: "example.com",
  type: "A",
  name: "@",
  value: "192.0.2.1",
  ttl: 3600,
  proxied: true,
  active: true,
  propagated: true,
  last_checked: ISODate("2024-02-12T10:30:00Z"),
  created_by: ObjectId("650a1b2c3d4e5f6789fedcba")
}
```

---

## Quick Reference: Agent's Main Loop

```go
func main() {
    // 1. Connect to MongoDB
    mongoClient := connectMongoDB()
    
    // 2. Load initial configuration
    config := loadInitialConfig(mongoClient)
    
    // 3. Start HTTP/HTTPS servers with loaded config
    router := buildRouter(config)
    go startHTTPServer(router)
    go startHTTPSServer(router)
    
    // 4. Start background tasks
    go runHealthChecks(config.ProxyConfigs)
    go syncStatistics(mongoClient)
    
    // 5. Watch Change Streams
    stream := watchCollections(mongoClient, ["domains", "dnsrecords", "proxyconfigs"])
    
    for event := range stream {
        applyConfigurationChange(event, router)
    }
}
```

---

## Summary

- **4 collections**: domains, teams, dnsrecords, proxyconfigs
- **Change Streams**: Real-time configuration updates
- **Team-based**: All configs scoped to team_id
- **Stateless agents**: No shared state, all config from MongoDB
- **Write-back stats**: Periodically sync request counters
- **WAF**: Custom expression language with priority-based evaluation
- **Reverse Proxy**: Full load balancing with health checks

The agent should be designed as a stateless service that can be horizontally scaled. Each agent instance watches the same MongoDB collections and independently applies configurations.
