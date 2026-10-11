---
name: go-house-style
description: Use when writing, reviewing or refactoring Go code under this team's house style. Covers KISS, TDD with a cleanup loop that prunes tests, SOLID in Go, dependency injection without patching, and strategies where they improve the code. Idiomatic Go standards live in go-development.
---

# Go House Style

These rules are the team's opinions, not Go standards. Each has a reason, stated with the rule.
Idiomatic Go lives in [`go-development`](../go-development/SKILL.md). Follow both, and if a rule
here ever contradicts it, fix this skill.

## KISS

The simplest code that meets the requirement wins. Go rewards plain code, and every abstraction is
code someone must read and maintain.

- No abstraction until a second use exists. A little duplication costs less than the wrong
  abstraction.
- No interface until something needs one. No option, config key or feature the task did not ask for.
- A function beats a type, and a type beats an interface with one implementation.
- A `switch` beats a strategy when the variants are few and stable, see
  [Strategy Pattern](#strategy-pattern--where-it-improves-the-code).
- When two designs both work, pick the one a new reader understands first.

## Development Loop — TDD

Red, Green, Refactor. The refactor step covers the tests too, because a suite that only grows ends
up slow, brittle and ignored.

1. **Red.** Write one failing test for the next behaviour, through the public API. Run it and see
   it fail for the expected reason.
2. **Green.** Write the minimum code that passes.
3. **Refactor production code and tests.**
   - Merge cases that exercise the same path into one table-driven test.
   - For every test you touch, ask: what plausible bug does this catch that no other test
     catches? No answer means delete it.
   - When a helper is an implementation detail, replace its tests with tests of the behaviour it
     serves.
   - Test count is not a metric. A feature PR may end with fewer tests than it started with.
   - A test that pins a fixed bug stays until the behaviour it pins is removed. Its value is the
     history, not the code path it exercises.

No production code without a failing test first. Exceptions: wiring in the composition root and
pure type declarations. A bug fix starts with a test that reproduces the bug.

## SOLID in Go

Go has no inheritance, so each principle shows up as a rule about packages and interfaces.

| Principle | Rule |
|-----------|------|
| Single responsibility | One reason to change per package and per type. An HTTP handler that computes a price breaks it. Business logic does not log or read configuration: it returns errors and events, and the outer layers handle them. |
| Open/closed | Behaviour that varies and grows is extended by adding a strategy, not by editing a `case`. See [Strategy Pattern](#strategy-pattern--where-it-improves-the-code). |
| Liskov substitution | When an interface has more than one real implementation, they all pass the same contract suite, e.g. `storetest.Run(t, newStore)` run against the real store and the in-memory fake. |
| Interface segregation | Interfaces are small and declared in the consumer package, listing only the methods it calls. Accept interfaces, return structs. |
| Dependency inversion | Packages holding business rules never import infrastructure such as `database/sql`, `net/http` or `os`, and never read `time.Now`. Infrastructure packages depend on them, never the reverse. Enforce the direction with `depguard`. |

## Dependency Injection — Never Patch

Code that reads a global, the clock or the environment cannot be tested without patching, and a
patched global breaks parallel tests and hides what the code depends on. So every collaborator that
does I/O, reads configuration, reads the clock or produces randomness is a constructor parameter.

1. `NewX(deps...) *X` takes each collaborator explicitly.
2. Real collaborators are built in one place, the composition root: `main`, or an `app.New(cfg)` it
   calls.
3. Tests pass doubles through the same parameters. In order of preference:
   - a real object configured for tests: `t.TempDir()`, in-memory SQLite, `httptest.NewServer`,
     `fstest.MapFS`, `rand.New(rand.NewPCG(seed, seed))`, a fixed clock
   - a hand-written fake implementing the interface
   - a generated mock (mockery, gomock), last resort only
4. Assert on return values and on what the fake recorded, never on unexported fields.

Randomness is an injected, seeded `*rand.Rand` from `math/rand/v2`, one per goroutine, and time is
an injected `Clock`. Business code never calls the global `rand` functions or `time.Now`.

```go
// BAD — hidden dependencies: tests must swap package vars and set env vars
var httpGet = http.Get

type QuoteSync struct{}

func (QuoteSync) Run() error {
    resp, err := httpGet(os.Getenv("QUOTES_URL"))
    // ...
    stamp := time.Now()
    // ...
}

// GOOD — dependencies are parameters; production wiring lives in the composition root
type Clock func() time.Time

type QuoteWriter interface {
    WriteQuotes(ctx context.Context, quotes []Quote, at time.Time) error
}

type QuoteSync struct {
    client  *http.Client
    baseURL string
    store   QuoteWriter
    now     Clock
}

func NewQuoteSync(client *http.Client, baseURL string, store QuoteWriter, now Clock) *QuoteSync {
    return &QuoteSync{client: client, baseURL: baseURL, store: store, now: now}
}

func (s *QuoteSync) Run(ctx context.Context) error {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/quotes/latest.json", nil)
    if err != nil {
        return fmt.Errorf("build quote request: %w", err)
    }
    // ... decode into quotes ...
    return s.store.WriteQuotes(ctx, quotes, s.now())
}
```

```go
// Test — no patching: a real HTTP server, a fake writer, a fixed clock
func TestQuoteSyncStampsQuotesWithInjectedClock(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        io.WriteString(w, `[{"symbol":"ABC","cents":125}]`)
    }))
    t.Cleanup(srv.Close)
    store := &fakeQuoteWriter{}
    fixed := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)

    sync := NewQuoteSync(srv.Client(), srv.URL, store, func() time.Time { return fixed })

    if err := sync.Run(context.Background()); err != nil {
        t.Fatal(err)
    }
    if !store.writtenAt.Equal(fixed) {
        t.Errorf("writtenAt = %v, want %v", store.writtenAt, fixed)
    }
}
```

These are patching in Go, and they are forbidden:

- a package-level function or variable reassigned by tests (`var now = time.Now`)
- mutable globals and singletons
- `init()` with side effects
- `os.Getenv` outside the composition root, or `t.Setenv` used to steer code that should take
  config as a parameter
- runtime patching libraries (gomonkey, bouk/monkey)
- test-only build-tag hooks in production code

In code you write or modify, refactor the production code to take the dependency as a parameter.
When reviewing, report it as a design issue in the production code. Leave untouched tests alone.

### Typing an injected dependency

| The dependency is… | Type it as |
|--------------------|------------|
| one behaviour, a few positional arguments | a named func type: `type Clock func() time.Time` |
| several methods, or stateful | a small interface declared in the consumer package |
| configured by many optional values | an options struct, not variadic `...any` |

`any` is never a dependency type.

## Strategy Pattern — Where It Improves the Code

A [strategy](https://en.wikipedia.org/wiki/Strategy_pattern) is one interface, or func type, with
one implementation per variant. Use it where it makes the code simpler or safer to extend, not by
reflex.

**Use a strategy when:**

- variants carry their own state or dependencies
- new variants keep arriving, so the set is open to extension
- the variant is selected from data, such as a config value or a database column
- the same `switch` on a kind is repeated in several places

**Use a `switch` when** there are a handful of stable variants and one place that chooses between
them. A `switch` over a typed constant, checked by the `exhaustive` linter, fails at lint time when
a case is missing. A registry only fails at runtime, so a strategy gives up that guarantee and must
earn it.

**Mechanics:**

- Selection happens once: at composition time, or from data through a registry keyed by a typed
  ID. Business code only calls the interface.
- An `if method == "express"` next to a registry, or a `switch` on the kind outside the registry,
  is a smell to refactor.
- Each strategy has its own table-driven tests. The registry has one test asserting that every
  known key resolves.

```go
// ShippingCalculator prices the delivery of one parcel.
type ShippingCalculator interface {
    Quote(parcel Parcel) Cents
}

type standardCalculator struct{} // flat rate by weight band
type expressCalculator struct{}  // standard plus a priority surcharge

type Checkout struct {
    calculators map[ShippingMethod]ShippingCalculator
}

func NewCheckout(calculators map[ShippingMethod]ShippingCalculator) *Checkout {
    return &Checkout{calculators: calculators}
}
```

## Review Checklist

- The simplest design that meets the requirement; no abstraction, option or interface without a use
- Each behaviour change started from a failing test; the refactor step pruned or merged redundant
  tests, and kept the tests that pin fixed bugs
- No patching in any form listed in [Dependency Injection](#dependency-injection--never-patch)
- Business-rule packages import no infrastructure package and do not log
- Varying behaviour is a strategy only where the criteria above hold, otherwise an exhaustive
  `switch`
- Interfaces with several implementations pass one contract suite
