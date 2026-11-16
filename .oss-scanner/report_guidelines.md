# How we would like reports written

- Write in Simplified Chinese, keeping identifiers, paths, commands and requests verbatim.
- One root cause per report, one to two pages. Start the title with the component in square brackets, for example
  `[middleware] 安全入口校验可被绕过`.
- Use these sections, in this order: 概述, 影响, 攻击者起点, 复现, 根因, 修复建议.
- State the attacker's starting position in the terms of `threat_model.md`, and whether a valid session or API token
  is needed at any step.
- State the affected file and function, the HTTP method and path that reach it, and any non-default configuration the
  finding depends on.
- Give a reproduction that runs as is: a Go test in the affected package, or exact `curl` commands. Say whether it was
  actually run and what it printed.
