# How we would like patches made

`AGENTS.md` at the repository root describes the architecture and conventions in full. The points below are the ones
that matter most for a security fix.

## Size and scope of a fix
- Fix the root cause in the layer that owns it, and cover the sibling paths that share it. Prefer a change that removes
  the edge case over one that adds another condition.
- Do not keep wrong behaviour for backward compatibility.
- Solve the reported problem only: no unrelated refactoring, renaming or reformatting, and no code for hypothetical
  scenarios.
- Respect the layering `route -> service -> biz <- data`: `internal/service` parses requests and shapes responses and
  holds no business logic; database, file and system-command access belongs in `internal/data`.
- Web server configuration goes through the dialects in `pkg/webserver` (`webserver.Get(type)`); do not build nginx,
  Apache, Caddy or OpenLiteSpeed configuration by hand elsewhere.
- Request validation belongs in the request structs of `internal/request` (validation tags) and the custom rules of
  `internal/rule`.
- No new dependencies unless unavoidable. Use the standard library and what is already in `go.mod`
  (`github.com/samber/lo` for collection handling).
- A new model field needs a migration appended to the end of `internal/migration/v1.go`.

## Code style
- Go code must pass `gofmt` and `GOOS=linux golangci-lint run ./...`. Many files carry `//go:build linux`, so build and
  lint with `GOOS=linux`.
- Comments are written in Simplified Chinese, state only a non-obvious reason, and fit on one line. A `//nolint:xxx`
  directive carries no explanation after it.
- A helper that serves a single type is an unexported method of that type, not a package-level function. Put a new
  declaration where it belongs in the file, not at the end.
- Text returned to the client or written to a log goes through `t.Get("english text")`, with a string literal as the
  message id. Do not edit the translation files under `pkg/embed/locales` and `web/src/locales`.
- Never log passwords, keys or tokens.
- When a constructor or an interface in `internal/biz` changes, regenerate the wiring with `go generate ./cmd/...` and
  the mocks with `go run github.com/vektra/mockery/v3@v3.9.0`, and include the generated files in the patch. If they
  cannot be regenerated in the scanner environment, say so in the report.
- Frontend code lives in `web/` (Vue 3, Naive UI). Wrap user-visible text in `$gettext('English text')`, and keep
  `pnpm type-check` and `pnpm lint` passing.

## Tests
- Add a regression test that fails before the fix and passes after it, in a `_test.go` file of the package that
  contains the fix.
- Use `check` and `must` from `github.com/libtnb/assert` for assertions. The mocks are in `internal/mocks`; calling a
  mock method that has no expectation panics, which is how "must not be called" is asserted.
- Run the tests of the affected package, then `go test ./...`. The frontend has no unit tests.

## Branch and commit message
- Against the `main` branch.
- The commit message is written in Simplified Chinese, in the form `type(scope): 概述`, for example
  `fix(middleware): 修复安全入口校验可被绕过`. The body lists, point by point, the changes a user can notice.
