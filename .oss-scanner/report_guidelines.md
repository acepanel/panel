# How we would like reports written

## Language
- Write the report in Simplified Chinese.
- Keep identifiers, file paths, commands, HTTP requests, log lines and error messages verbatim, in their original form.

## Layout and length
- Keep each report readable in a few minutes: one to two pages.
- Use these sections, in this order: 概述, 影响, 攻击者起点, 受影响位置, 前置条件, 复现, 根因, 修复建议.
- 概述 is two or three sentences: what an attacker can do, and through which entry point.

## What every report should state
- The attacker's starting position, using the classes from `threat_model.md`: unauthenticated network client, a web
  page visited by a logged-in administrator, a source of untrusted data (website visitor, SSH client, remote server the
  panel talks to), a local unprivileged user (say which one, for example `www`), or the holder of a webhook key or a
  file-share token.
- Whether a valid panel session or API token is needed at any step. If one is needed, say exactly which boundary the
  finding still crosses.
- The affected code: file path and function (for example `internal/middleware/entrance.go`, `entrance`), the layer it
  belongs to (`internal/route`, `internal/service`, `internal/biz`, `internal/data`, `internal/apps/<slug>`,
  `pkg/<name>`, `web/src/...`), and the HTTP method and path that reach it.
- The preconditions: every non-default value needed in `config.yml` (`http.entrance`, `http.bind_domain`, `http.bind_ip`,
  `http.bind_ua`, `http.ip_header`, `http.tls`, `http.login_captcha`, `app.debug`), every panel setting, which web
  server or sub-app must be installed, and the distribution if it matters.
- The commit of `main` the finding was made against.
- A reproduction a maintainer can run as is. Prefer a Go test placed in the affected package. Otherwise give the exact
  `curl` commands, including the login steps when a session is involved, with the expected and the observed result.
  For a frontend finding, give the exact input and the page on which it renders.
- Whether the reproduction was actually run in the scanner environment and what it printed, or that it is untested.
- The severity according to `threat_model.md`, with one sentence of justification.

## Style
- One root cause per report. If several entry points reach the same faulty code, list them all in that one report.
- Start the title with the component in square brackets, for example `[middleware] 安全入口校验可被绕过`.
- Plain, factual language. Say clearly when an impact is inferred rather than demonstrated.
- Put code, requests and output in code blocks, and trim long output to the relevant lines.
