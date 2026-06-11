# Security Policy

## Reporting a vulnerability

Please report security vulnerabilities **privately** — do not open a public GitHub
issue.

- **Preferred:** use GitHub's [private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
  on this repository (Security → Report a vulnerability).
- **Or email:** security@gamenest.example *(replace with a real, monitored address
  before launch)*.

Please include steps to reproduce, the affected version (Settings → About), and the
potential impact. We aim to acknowledge reports within a few days.

## Scope

GameNest runs locally and binds its engine to `127.0.0.1` by default; game servers
run in isolated Docker containers. Of particular interest:

- The engine HTTP/WebSocket API (`engine/internal/api`) and its auth
  (`engine/internal/auth`, session handling, loopback trust).
- The optional remote-access HTTPS listener (`engine/internal/remote`) and TLS
  handling (`engine/internal/tlscert`).
- Container creation/arguments (`engine/internal/docker`) and path handling for
  files/backups.
- Secret storage (`engine/internal/secret`, DPAPI).

## Supported versions

GameNest is pre-1.0 and ships frequently; security fixes target the **latest**
release. Please update to the newest version before reporting.
