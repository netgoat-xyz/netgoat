# Security policy

NetGoat is active alpha software. Security fixes are applied to the current development branch; older snapshots may not receive backports.

## Reporting a vulnerability

Do not disclose a suspected vulnerability in a public issue, discussion, chat, or pull request. Email **duckeydev@gmail.com** with:

- the affected component and revision;
- the impact and conditions required to exploit it;
- reproducible steps or a minimal proof of concept, when safe to provide;
- any suggested mitigation; and
- a secure way to contact you.

Avoid accessing data that is not yours, disrupting public services, or retaining sensitive data while researching. A maintainer will acknowledge the report as capacity permits, validate it, prepare a fix, and coordinate disclosure with the reporter. Please allow time for a patch before publishing details.

Reports about directly exploitable dependency behavior, unsafe defaults, authentication or proxy-boundary mistakes, denial of service, and sensitive-data exposure are in scope. General support requests, UI defects without security impact, and scanner output without a plausible impact are better filed through the normal issue tracker.

## Deployment guidance

- Use TLS for public traffic and keep control-plane or telemetry administration on loopback or behind an authenticated TLS proxy.
- Do not use placeholder secrets. Bootstrap users explicitly with a unique password of at least 12 characters.
- Configure `trusted_proxies` narrowly; forwarding headers are ignored by default.
- Keep SQLite databases, recovery snapshots, `.env`, private keys, telemetry IDs, and AI model artifacts out of source control.
- Leave optional telemetry disabled unless you operate and trust its configured destination.
