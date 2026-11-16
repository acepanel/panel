# AcePanel threat model

AcePanel is a Linux server management panel. One Go binary (`cmd/ace`) runs as root and serves a web UI and a JSON
API; the Vue frontend in `web/` is embedded in it. `cmd/cli` is a root-only command line tool. See `SECURITY.md` for
the security policy and `AGENTS.md` for the architecture.

## Scope rule

An authenticated administrator is root by design. Every panel user is a full administrator, and running commands and
editing any file is what the product is for. Anything that needs a valid session or API token is therefore not a
vulnerability: command injection, path traversal, file read or write, upload, SQL injection and SSRF in authenticated
endpoints are intended capability. Please do not report them.

In scope is whatever lets someone who is not an administrator influence the panel or the host.

## Attackers

1. **Unauthenticated network client.** It reaches the endpoints marked `Public: true` in `internal/route`, the paths
   outside `/api` (webhooks, file-share downloads, static files), and the checks in `internal/middleware`: entrance
   path, bind restrictions, login, throttling, sessions, two-factor codes, passkeys and API token signatures.
2. **Cross-site page** opened by a logged-in administrator: CSRF, XSS, DNS rebinding and cross-origin WebSocket
   connections. The WebSocket endpoints include a root terminal.
3. **Untrusted data the panel ingests**: website visitors through access logs, SSH clients through authentication
   logs, and remote servers the panel talks to. This data must not crash the panel, exhaust memory or disk, or run as
   script when an administrator views it.
4. **Local unprivileged user**, typically `www` running a compromised website. It must not become root, read the
   panel's secrets in `/opt/ace/panel/storage`, or reach other websites through what the panel does as root in
   directories that user can write, through the world-writable statistics socket, through generated configuration, or
   by defeating tamper protection.
5. **Holder of a webhook key or a file-share token.** It is limited to that one script or that one file.

## Out of scope

- Anything that needs a session, an API token or root on the host, and the `cli` binary.
- Debug mode (`app.debug: true`), which is for development only.
- Spoofing the header named by `http.ip_header`: setting it declares a trusted proxy in front of the panel.
- A compromise of the official update and app endpoints. A flaw in how the panel verifies them is in scope.
- Bugs in the managed software and in dependencies, unless the panel's own code or generated configuration makes
  them reachable.
- Generated files (`internal/mocks`, `wire_gen.go`, `pkg/embed/frontend`), self-XSS, and missing security headers
  without a concrete attack.

## Severity

- **Critical**: attacker 1 or 2 gains command execution, authentication bypass, a forged session or API signature, or
  arbitrary file read or write.
- **High**: stored XSS in the UI from attacker 3; attacker 4 becomes root or reads the panel's secrets; two-factor or
  passkey bypass; a webhook key or share token reaching beyond its target; unauthenticated disclosure of credentials.
- **Medium**: bypass of the entrance path, of a bind restriction or of login throttling on its own; unauthenticated
  denial of service; reflected XSS; bypass of tamper protection; weak isolation between websites caused by generated
  configuration.
- **Low**: hardening gaps without a demonstrated attack.

## Running it

The binaries are in `/opt/ace/panel` and the configuration is `storage/config.yml` (plain HTTP, port 8888).

```sh
cd /opt/ace/panel && ./ace >/tmp/ace.log 2>&1 &
ACEPANEL_PASSWORD='Scanner-Test-1' ./cli user create scanner
```

To log in, `GET /api/user/key` returns an RSA public key bound to the session cookie, and `POST /api/user/login` takes
`username` and `password`, each encrypted with RSA-OAEP (SHA-512) and base64-encoded. The image has no systemd and none
of the managed software, so pages that drive them return errors. Tests run with `go test ./...` in `/src`; the ones
that need the Internet fail offline.
