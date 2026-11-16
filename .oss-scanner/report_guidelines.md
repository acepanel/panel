# How we would like reports written

- One root cause per report, one to two pages. Start the title with the component in square brackets, for example
  `[middleware] entrance check can be bypassed`.
- Use these sections, in this order: Summary, Impact, Attacker position, Reproduction, Root cause, Suggested fix.
- State the attacker's starting position in the terms of `threat_model.md`, and whether a valid session or API token
  is needed at any step.
- State the affected file and function, the HTTP method and path that reach it, and any non-default configuration the
  finding depends on.
- Give a reproduction that runs as is: a Go test in the affected package, or exact `curl` commands. Say whether it was
  actually run and what it printed.
