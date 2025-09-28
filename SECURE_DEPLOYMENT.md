# Secure Deployment Guide

## Enforcing HTTPS

All NetGoat services must be deployed behind HTTPS. HTTP access to sensitive endpoints is strictly disallowed.

### Steps to Enable HTTPS

1. **Certificates**: Obtain SSL/TLS certificates for your domain (e.g., via Let's Encrypt).
2. **Server Configuration**: Ensure all services (LogDB, CentralMonServer, ShardManager) are started with HTTPS options and valid certs. See environment variables:
   - `SSL_KEY_PATH`: Path to your private key
   - `SSL_CERT_PATH`: Path to your certificate
3. **Reverse Proxy (Recommended)**: Use NGINX or Caddy to terminate HTTPS and forward requests to backend services. Redirect all HTTP traffic to HTTPS.
4. **Firewall**: Block port 80 (HTTP) except for certificate renewal. Only expose port 443 (HTTPS).

### Example NGINX Config
```nginx
server {
    listen 80;
    server_name yourdomain.com;
    return 301 https://$host$request_uri;
}
server {
    listen 443 ssl;
    server_name yourdomain.com;
    ssl_certificate /etc/ssl/certs/server.crt;
    ssl_certificate_key /etc/ssl/private/server.key;
    location / {
        proxy_pass http://localhost:3010; # or your service port
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $remote_addr;
    }
}
```

### Client Usage
- Always use `https://` URLs for API and asset requests.
- Browsers and scripts should reject insecure HTTP endpoints.

### Sensitive Endpoints
- All authentication, data, and admin endpoints must be accessed via HTTPS only.

## Additional Notes
- Update any hardcoded URLs in your codebase to use HTTPS.
- For local development, use self-signed certificates or tools like mkcert.
- See updated server configs for HTTPS enforcement.
