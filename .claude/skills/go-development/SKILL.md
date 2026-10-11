---
name: go-development
description: Use when writing, reviewing, testing or refactoring Go code. Covers idiomatic Go as the community practices it, namely naming, doc comments, errors, context, concurrency, API design, domain modelling, HTTP and SQL, slog, testing, modern Go features and tooling. Team opinions live in go-house-style.
---

# Go Development

Idiomatic Go, as written down by the Go community. The sources are
[Effective Go](https://go.dev/doc/effective_go),
[Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), the
[Google Go Style Guide](https://google.github.io/styleguide/go/) and the
[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).

This skill holds standards, not preferences. Team opinions such as TDD, SOLID and strategies are in
[`go-house-style`](../go-house-style/SKILL.md). Follow both.

Read the `go` directive in `go.mod` before using a language or library feature, see
[Modern Go](#modern-go).

## Formatting and Naming

- Format with `gofumpt` and `goimports`. Go has no fixed line length: break lines for readability,
  not to reach a count.
- MixedCaps, never underscores. Initialisms keep one case: `ID`, `URL`, `HTTP`, `userID`.
- Packages are short, lowercase and singular. A name does not repeat its package: `booking.Reservation`,
  not `booking.BookingReservation`.
- No `utils`, `common` or `helpers` packages. Name a package for what it provides.
- Receivers are one or two letters and consistent across a type's methods.
- No `Get` prefix on getters. A one-method interface is named after the method with `-er`.

## Documentation

- Every exported name has a doc comment, a full sentence starting with the name.
- A package has a package comment that says what it is for.
- Comments say why, not what. Do not restate the code.

## Errors

- Return errors. `panic` is for programmer errors and impossible states, never for expected
  failures.
- Wrap with `%w` only when callers may depend on the cause, because the wrapped error becomes part of
  your API. Use `%v` to hide it.
- Match with `errors.Is` and `errors.As`, never by comparing strings or with `==` on wrapped errors.
- Error strings are lowercase with no trailing punctuation, and say what failed:
  `fmt.Errorf("load config: %w", err)`.
- Handle an error once. Either return it or log it, never both.
- Use sentinel errors (`var ErrNotFound = errors.New(...)`) for conditions callers branch on, and an
  error type when callers need data from it. `errors.Join` combines independent failures.
- A `Must` helper that panics belongs only in program start-up, such as compiling a constant
  regexp.

## Context

- `ctx context.Context` is the first parameter of anything that does I/O or can block.
- Do not store a context in a struct. Pass it through calls.
- Honour cancellation: check `ctx.Done()` in loops and pass `ctx` to every call that accepts one.
- `context.Background()` appears only at the top of a program or in tests.

## Concurrency

- Every goroutine has an owner that knows how it stops. Do not start a goroutine without answering
  how it ends.
- Tie goroutine lifetime to a `ctx` and wait for it before returning. `errgroup` or `sync.WaitGroup`
  does this.
- Share memory by communicating when that reads clearly. Guard shared state with a `sync.Mutex`
  when it does not.
- Document which fields a mutex guards, and never copy a type that contains one.
- A seeded `*rand.Rand` is not safe for concurrent use. Give each goroutine its own, derived from
  the seed.
- Recover from panics at a goroutine boundary only if the program must keep running, and log the
  panic.
- Test concurrent code with `go test -race`. Leaked goroutines are bugs: check for them in tests of
  anything that starts one.

## API Design

- Make the zero value useful when it can be, and have a constructor when it cannot.
- Accept interfaces, return concrete types. Declare an interface in the package that uses it, with
  only the methods it calls.
- Do not define an interface before there is a use for it. A concrete type is easier to read and to
  change.
- Avoid mutable package-level state and `init()` with side effects.
- Copy slices and maps that cross a package boundary if the receiver might mutate them.
- Use `time.Duration` for durations, never a bare integer.
- Start `iota` enums at 1 so the zero value means "unset".
- Keep generics for code that is truly type-agnostic. A concrete function is clearer for one type.
- Do not embed a type in an exported struct just to save typing: it exports the embedded methods.
- Release resources with `defer` right after acquiring them. Do not `defer` inside a loop.
- Optional parameters go in an options struct, or in functional options when the set grows.

## Domain Modelling

- Closed sets are typed constants, checked by the `exhaustive` linter.
- Variants carrying data are sealed interfaces, with an unexported marker method, checked by
  `gochecksumtype`.
- Constructors validate, and fields stay unexported when an invariant depends on them.
- Money is an integer count of the smallest unit (`int64`), never `float64`.
- Check for overflow where sums are unbounded.

## HTTP and SQL

- Never use `http.DefaultClient` for outbound calls: it has no timeout. Build a client with a
  timeout, or a context deadline on every request.
- Set `ReadHeaderTimeout` at least on every `http.Server`.
- Always close response bodies, and drain them if the connection should be reused.
- Build requests with `http.NewRequestWithContext`.
- Use parameterised queries. Never build SQL by concatenating or formatting input.
- Close `rows`, check `rows.Err()`, and `defer tx.Rollback()` right after beginning a transaction.
- Use the `...Context` variants of `database/sql` calls.
- Render untrusted text with `html/template`, never by writing it into HTML yourself.
- Use `crypto/rand` for tokens and secrets, never `math/rand`.
- Resolve untrusted file paths with `filepath.IsLocal`, or `os.Root` to confine access to a
  directory.

## Logging with `log/slog`

- Use `log/slog` for diagnostics. Never `fmt.Print*` or `log.Print*`.
- Logs are structured key-value pairs: `logger.Info("order placed", "customer", id)`. The message is
  a constant string, and variable data goes in attributes.
- Use levels as intended. Errors are logged at `Error`, once, where they are handled.
- Never log secrets or personal data.

## Testing

- Use the standard `testing` package, with `go-cmp` for diffs. Do not add an assertion library.
- Table-driven tests with a `name` field and `t.Run(tc.name, ...)`.
- Failure messages say what happened: `got %v, want %v`, with the function and inputs when they
  help.
- Mark helpers with `t.Helper()`. Clean up with `t.Cleanup`, and use `t.TempDir()` for files.
- Keep fixtures in `testdata/`. Use golden files for large expected output.
- Tests of the public API live in an external test package (`package invoice_test`). Test
  unexported behaviour from the package itself, and only when the public API cannot reach it.
- Use `t.Parallel()` when tests share no state, and run with `-race` and `-shuffle=on` so hidden
  dependencies on order show up.
- No `time.Sleep` to wait for something. Use `testing/synctest` or an injected clock.
- Add a fuzz test to anything that parses external input.
- Benchmark with `b.Loop()`, and measure before optimising.

## Modern Go

Check the `go` directive first, then prefer the newer form. Versions are from the Go release notes.

| Instead of | Use | Since |
|------------|-----|-------|
| `for i := 0; i < n; i++` | `for i := range n` | 1.22 |
| Re-declaring the loop variable | Nothing: each iteration has its own variable | 1.22 |
| Hand-written min, max, clear | The `min`, `max` and `clear` built-ins | 1.21 |
| Hand-written slice and map helpers | `slices`, `maps`, `cmp` | 1.21 |
| Callback-style iteration | Range over functions with `iter`, used sparingly | 1.23 |
| `context.WithCancel` in tests | `t.Context()` | 1.24 |
| `for i := 0; i < b.N; i++` | `b.Loop()` | 1.24 |
| Sleeping or fake clocks in concurrent tests | `testing/synctest` | 1.25 |
| `wg.Add(1)` and `go func() { defer wg.Done() }` | `wg.Go(...)` | 1.25 |
| Path checks by hand | `os.Root` | 1.24 |
| Tools installed outside the module | The `tool` directive in `go.mod`, run with `go tool` | 1.24 |

## Tooling

- `golangci-lint run` passes. The enabled linters are listed in the repository's golangci
  configuration, which is the source of truth. Each rule in this skill that a linter can enforce
  should be enforced by it.
- `go vet`, `go test -race ./...` and `govulncheck ./...` pass.
- Keep dependencies few. Commit `go.sum`. `go mod tidy` leaves no diff.
- Major versions live in the import path, so a major upgrade is always an explicit change.

| Linter | Enforces |
|--------|----------|
| `errcheck`, `staticcheck`, `unused`, `ineffassign`, `govet` | Basic correctness |
| `revive` | Naming and doc comments |
| `errorlint` | `errors.Is`, `errors.As` and `%w` |
| `noctx` | Requests built with a context |
| `bodyclose`, `sqlclosecheck`, `rowserrcheck` | Closed bodies, statements and rows |
| `exhaustive`, `gochecksumtype` | Closed sets and sealed variants, see [Domain Modelling](#domain-modelling) |
| `gochecknoglobals`, `gochecknoinits` | No mutable package state and no `init()` |
| `sloglint` | Constant messages and consistent attributes |
| `forbidigo` | No `fmt.Print*` and no `log.Print*` for diagnostics |
| `depguard` | Import direction between packages |
| `thelper`, `tparallel` | `t.Helper()` and correct parallel tests |

## Review Checklist

- Names, doc comments and package names follow the conventions above
- Errors are returned once, wrapped deliberately and matched with `errors.Is` or `errors.As`
- Every I/O function takes `ctx` first, and no context is stored in a struct
- Each goroutine has an owner and a way to stop; shared state is guarded; `-race` passes
- Interfaces are small, declared by their consumer, and exist because something uses them
- HTTP clients and servers have timeouts; SQL is parameterised; bodies and rows are closed
- Money is an integer type
- Tests have useful failure messages, no `Sleep`, and pass with `-race -shuffle=on`
- `golangci-lint run` passes and `go mod tidy` leaves no diff
