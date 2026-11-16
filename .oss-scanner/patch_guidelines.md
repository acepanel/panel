# How we would like patches made

- Follow `AGENTS.md` at the repository root. It holds the architecture and the code conventions.
- Fix the root cause and the sibling paths that share it, with no unrelated changes.
- Add a regression test that fails before the fix and passes after it.
- Work against `main`. Write the commit message in English, in the form `type(scope): summary`.
