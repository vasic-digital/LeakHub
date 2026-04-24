# CLAUDE.md -- digital.vasic.leakhub


## Definition of Done

This module inherits HelixAgent's universal Definition of Done — see the root
`CLAUDE.md` and `docs/development/definition-of-done.md`. In one line: **no
task is done without pasted output from a real run of the real system in the
same session as the change.** Coverage and green suites are not evidence.

### Acceptance demo for this module

<!-- TODO: replace this block with the exact command(s) that exercise this
     module end-to-end against real dependencies, and the expected output.
     The commands must run the real artifact (built binary, deployed
     container, real service) — no in-process fakes, no mocks, no
     `httptest.NewServer`, no Robolectric, no JSDOM as proof of done. -->

```bash
# TODO
```

Module-specific guidance for Claude Code.

## Status

**FUNCTIONAL.** 2 packages (types, client) ship tested implementations;
`go test -race ./...` all green. Default signature list (10 canonical
leak phrases) seeded on `New()`.

## Hard rules

1. **NO CI/CD pipelines** -- no `.github/workflows/`, `.gitlab-ci.yml`,
   `Jenkinsfile`, `.travis.yml`, `.circleci/`, or any automated
   pipeline. No Git hooks either. Permanent.
2. **SSH-only for Git** -- `git@github.com:...` / `git@gitlab.com:...`.
3. **Conventional Commits** -- `feat(leakhub): ...`, `fix(...)`,
   `docs(...)`, `test(...)`, `refactor(...)`.
4. **Code style** -- `gofmt`, `goimports`, 100-char line ceiling,
   errors always checked and wrapped (`fmt.Errorf("...: %w", err)`).
5. **Resource cap for tests** --
   `GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -count=1 -p 1 -race ./...`

## Purpose

Prompt-leak corpus + signature-match detection. Key surface:
`DetectLeak`, `SearchArchive`, `AddToArchive`, `GetByModel`,
`GetStats`, `ExportArchive`, `AddSignatures`, `Count`.

## Primary consumer

HelixAgent (`dev.helix.agent`) — red-team / guardrail subsystems.

## Testing

```
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -count=1 -p 1 -race ./...
```

## API Cheat Sheet

**Module path:** `digital.vasic.leakhub`.

```go
type LeakEntry struct {
    ID, Model, LeakText, Source, Date, Severity string
    Tags []string
}
type DetectionOptions struct {
    Text string
    CaseSensitive bool
    Threshold float64
}
type DetectionResult struct {
    Leaked bool
    Matches []LeakMatch
    Confidence float64
}
type LeakMatch struct {
    Signature, Context, Severity string
    Position int
}

type Client struct { /* archive + signature list */ }

func New(opts ...config.Option) (*Client, error)
func (c *Client) AddSignatures(sigs ...string)
func (c *Client) DetectLeak(ctx, text string) (*DetectionResult, error)
func (c *Client) SearchArchive(ctx, query string, limit int) ([]LeakEntry, error)
func (c *Client) AddToArchive(entry LeakEntry) error
func (c *Client) GetByModel(ctx, model string) ([]LeakEntry, error)
func (c *Client) GetStats(ctx) (*ArchiveStats, error)
func (c *Client) ExportArchive(ctx, format string) ([]byte, error)
func (c *Client) Count() int
func (c *Client) Close() error
```

**Typical usage:**
```go
c, _ := leakhub.New()
defer c.Close()
c.AddSignatures("my_secret_phrase")
r, _ := c.DetectLeak(ctx, modelOutput)
if r.Leaked { /* handle */ }
```

**Injection points:** none.
**Defaults on `New`:** 10 canonical leak phrases (`"you are an ai language model"`, `"system prompt:"`, etc.).

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | PliniusCommon |
| Downstream (these import this module) | root only |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.
