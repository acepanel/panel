# AcePanel threat model

AcePanel is a server management panel for Linux. A single Go binary (`cmd/ace`) runs as root, serves a web UI and a
JSON API on one HTTP(S) port, and manages websites, databases, containers, certificates, firewall rules, files,
terminals and scheduled tasks on the host. The Vue 3 frontend in `web/` is embedded into the binary. A second binary
(`cmd/cli`, installed as `acepanel`) is a root-only command line tool that shares the same code.

Because the panel is root on the machine it manages, any way to act as an administrator without being one is a full
host compromise.

Project security policy: `SECURITY.md`. Architecture and conventions: `AGENTS.md`.

## The one rule that decides scope

**An authenticated administrator is root by design.** Every panel user is a full administrator; there are no roles.
The panel's purpose is to let that user run commands, edit any file and reconfigure the system. Anything that needs a
valid session cookie or a valid API token to carry out is therefore not a vulnerability: command injection, path
traversal, arbitrary file read or write, file upload, SQL injection, SSRF and template injection in authenticated
endpoints are all intended capability. Please do not report them.

What matters is everything that lets someone who is **not** an administrator gain influence over the panel or the host.

## Attackers to consider

1. **Unauthenticated network client** that can reach the panel port.
2. **Cross-site attacker**: a web page opened by an administrator who is logged in to the panel. CSRF, cross-site
   WebSocket hijacking, XSS, clickjacking and DNS rebinding belong here. The WebSocket endpoints under `/api/ws/`
   include a root terminal, so a cross-origin connection to them is as bad as an authentication bypass.
3. **Sources of untrusted data that the panel ingests without authentication**: visitors of the hosted websites (URI,
   User-Agent, client IP headers and request bodies arrive through access logs), SSH clients (user names and addresses
   in the authentication log), hosts probing the firewall, and remote servers the panel talks to (ACME, DNS provider
   APIs, object storage, the update and app endpoints). This data must not crash the panel, exhaust its memory or
   disk, or execute as script when an administrator later views it in the UI.
4. **Local unprivileged user or process** on the managed host, typically the `www` user running a compromised
   website, or a database or container user. It must not become root, read panel secrets, or tamper with other sites
   through anything the panel does or leaves behind.
5. **Holder of a limited credential**: a webhook key may only trigger that webhook's script, and a file-share token may
   only download that one file within its expiry and download limit. Neither may reach anything else.

## Components and where untrusted input enters

- **Request pipeline** (`internal/middleware`): `entrance.go` enforces the secret entrance path and the bind
  restrictions (`http.bind_domain`, `http.bind_ip`, `http.bind_ua`); `must_login.go` enforces authentication for every
  path under `/api` that is not marked `Public: true` in `internal/route`; `throttle.go` rate-limits login. Paths
  outside `/api` skip the login check, and `/webhook/` and `/download/` also skip the entrance check.
- **Endpoints reachable without a session** (`internal/route`, handlers in `internal/service`): `/api/user/key`,
  `/api/user/captcha`, `/api/user/login`, `/api/user/logout`, `/api/user/is_login`, `/api/user/is_2fa`,
  `/api/user/passkey/enabled`, `/api/user/passkey/login`, `/api/home/panel`, `/webhook/{key}`, `/download/{token}`,
  and the static frontend.
- **Authentication** (`internal/service/user.go`, `user_passkey.go`, `login_guard.go`, `internal/data/user*.go`):
  password login with credentials wrapped in RSA-OAEP under a per-session key, captcha after repeated failures, TOTP
  two-factor codes, WebAuthn passkeys, server-side sessions, and an optional binding of the session to the client IP.
- **API tokens** (`internal/data/user_token.go`): requests signed with HMAC-SHA256
  (`Authorization: HMAC-SHA256 Credential=<id>, Signature=<sig>` plus `X-Timestamp`), with an expiry and an optional
  IP allowlist per token. Token authentication is refused on `/api/ws/`.
- **Client IP** (`pkg/clientip`): taken from the connection, or from the header named by `http.ip_header` when the
  administrator configures one. It feeds the bind restriction, login throttling, the session binding and the token
  allowlist.
- **Webhooks** (`internal/service/webhook.go`, `internal/data/webhook.go`): `/webhook/{key}` runs an
  administrator-defined script, as root by default, for anyone who knows the key.
- **File shares** (`internal/service/file_share.go`, `internal/data/file_share.go`): `/download/{token}` serves one
  file chosen by an administrator.
- **Website statistics** (`pkg/websitestat`, `internal/job/website_stat.go`): a Unix datagram socket at
  `/tmp/ace_stats.sock`, mode 0666, receives access-log JSON from the web servers. Any local user can write to it, and
  its content originates from website visitors. Parsed values are aggregated in memory, stored, and shown in the UI.
- **Log parsing** (`pkg/sshlog`, `internal/service/log.go`, `internal/biz/alert_ssh.go`): SSH authentication logs and
  other system logs contain attacker-chosen strings.
- **Kernel-facing code** (`pkg/tamper`, `pkg/firewall/scan`): eBPF programs for website tamper protection and for
  scan detection. Tamper protection is a security feature whose bypass by the `www` user is in scope.
- **Files in website directories**: the panel, as root, reads, writes, archives, restores, changes ownership of and
  serves paths that the `www` user can write. Symlink following and time-of-check/time-of-use races here are in scope
  (`pkg/io`, `internal/data/backup.go`, `internal/data/website.go`, `internal/service/file.go`).
- **Generated configuration** (`pkg/webserver`, `internal/apps/<slug>`): configuration the panel writes for nginx,
  Apache, Caddy, OpenLiteSpeed, PHP, databases and other managed software. A default that weakens isolation between
  websites or exposes a service to the network without the administrator asking for it is in scope.
- **Secrets at rest** (`/opt/ace/panel/storage`): `config.yml` holds the application key, `panel.db` holds password
  hashes, sessions, API tokens and stored credentials, and the directory holds the panel's TLS key and logs. Anything
  that leaves these, backups, or generated files with credentials readable by unprivileged users is in scope.
- **Frontend** (`web/src`): renders strings that come from the sources in attacker class 3. The places that inject HTML
  (`v-html`, `innerHTML`) deserve attention.

## Trusted, out of scope

- Authenticated administrators and API token holders, as described above, and anyone who is already root on the host
  or can write to `/opt/ace/panel`.
- The `cli` binary: only root can run it.
- Debug mode (`app.debug: true`): it disables the entrance check and the WebSocket origin check and publishes the API
  documentation. It is for development only.
- The official endpoints configured as `app.api_endpoint` and `app.download_endpoint`. The panel downloads updates and
  app install scripts from them over TLS and runs them as root. A flaw in how the panel verifies TLS or checksums is in
  scope; a hypothetical compromise of the endpoints is not.
- The header named by `http.ip_header`. When an administrator sets it, the panel is expected to sit behind a trusted
  proxy; spoofing that header against a directly exposed panel is a deployment mistake.
- Bugs in the software the panel installs and manages (nginx, PHP, MySQL, Docker and so on) and in third-party
  dependencies, unless the panel's own code or generated configuration makes them reachable.
- The installer and helper scripts, which live in other repositories.
- Generated and vendored content: `internal/mocks`, `cmd/*/wire_gen.go`, `pkg/embed/frontend` (analyse `web/src`
  instead), and the translation files under `pkg/embed/locales` and `web/src/locales`.
- Self-XSS, missing security headers without a concrete attack, and denial of service that needs a session.

## How we rate severity

- **Critical**: an unauthenticated network client or a cross-site page gains command execution, authentication bypass,
  session or API signature forgery, or arbitrary file read or write. This includes cross-origin access to `/api/ws/`.
- **High**: stored XSS in the panel UI from data in attacker class 3; a local unprivileged user becomes root or reads
  panel secrets; two-factor or passkey verification bypassed by someone who has the password; a webhook key or
  file-share token reaching beyond its script or file; bypass of an API token's expiry or IP allowlist;
  unauthenticated disclosure of password hashes, sessions, tokens or the application key.
- **Medium**: bypass of the entrance path or of a bind restriction on its own; bypass of login throttling or captcha
  that makes online password guessing practical; unauthenticated denial of service that crashes or hangs the panel or
  exhausts memory or disk with modest effort, including through the log ingestion paths; reflected XSS that needs a
  crafted link; bypass of tamper protection by the `www` user; isolation weaknesses between websites caused by
  generated configuration.
- **Low**: hardening gaps with a plausible but undemonstrated attack, and local denial of service.
- Do not rate anything that needs a valid session or API token; it is not a finding.

## How to exercise it

- The checkout is in `/src` (Go module `github.com/acepanel/panel/v3`). `internal/route` is the routing table,
  `internal/service` the handlers, `internal/biz` the domain logic and interfaces, `internal/data` the implementations,
  `internal/apps/<slug>` the sub-apps mounted at `/api/apps/<slug>`, and `pkg/` the standalone packages.
- Tests: `go test ./...` in `/src`. Some tests in `pkg/acme`, `pkg/api`, `pkg/ntp` and `pkg/tools` need the Internet
  and fail without it, and `pkg/tamper` loads eBPF programs, which needs a privileged container. Build and lint with
  `GOOS=linux`; many files carry `//go:build linux`.
- The built binaries are `/opt/ace/panel/ace` and `/opt/ace/panel/cli`, with a configuration at
  `/opt/ace/panel/storage/config.yml` (plain HTTP, port 8888, entrance `/`). To start a panel:

  ```sh
  cd /opt/ace/panel
  ./ace >/tmp/ace.log 2>&1 &                       # creates and migrates storage/panel.db, then listens
  ACEPANEL_PASSWORD='Scanner-Test-1' ./cli user create scanner
  curl -s http://127.0.0.1:8888/api/home/panel
  ```

- To log in from a script: `GET /api/user/key` returns an RSA public key bound to the session cookie; the user name and
  the password are each encrypted with RSA-OAEP (SHA-512) and base64-encoded:

  ```sh
  J=/tmp/cookies.txt
  curl -s -c $J http://127.0.0.1:8888/api/user/key | jq -r .data > /tmp/pub.pem
  enc() { printf '%s' "$1" | openssl pkeyutl -encrypt -pubin -inkey /tmp/pub.pem \
            -pkeyopt rsa_padding_mode:oaep -pkeyopt rsa_oaep_md:sha512 | base64 -w0; }
  curl -s -b $J -c $J -H 'Content-Type: application/json' \
    -d "{\"username\":\"$(enc scanner)\",\"password\":\"$(enc Scanner-Test-1)\",\"safe_login\":false,\"pass_code\":\"\",\"captcha_code\":\"\"}" \
    http://127.0.0.1:8888/api/user/login
  curl -s -b $J http://127.0.0.1:8888/api/user/info
  ```
- To test the entrance and bind restrictions, edit `http.entrance`, `http.bind_domain`, `http.bind_ip` and
  `http.bind_ua` in `config.yml` and restart `ace`.
- The image has no systemd and none of the managed software, so pages that drive nginx, databases or containers
  return errors. The request pipeline, authentication, sessions, API tokens, webhooks, file shares, the file manager,
  terminals and the statistics socket work without them.
