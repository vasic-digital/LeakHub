# LeakHub

Prompt-leak detection, extraction, and archival. Detects potential
system-prompt leaks in AI model responses and maintains a searchable
in-memory archive of known leaks. Part of the Plinius Go service
family used by the HelixAgent ensemble.

Module path: `digital.vasic.leakhub` — two packages: `pkg/types`
(value types) and `pkg/client` (detection + archive).

## Status

- **FUNCTIONAL** — both packages ship tested implementations.
- `go test -race -count=1 ./...` is green (round-266 evidence in
  `docs/test-coverage.md`).
- Default-signature list seeded on `New()`: 10 canonical leak
  phrases (`"you are an ai language model"`, `"system prompt:"`,
  `"ignore previous instructions"`, `"my system prompt"`, etc.).
- Detector preserves byte content: the `LeakMatch.Position` reported
  by `DetectLeak` is a byte offset into the original response, and
  `MatchedText` slices the response at that position — verified
  round-trip across 5 locales (en, sr, ja, ar, zh-CN) by the
  round-266 challenge runner.
- Integration-ready: consumable Go library for HelixAgent
  red-team / guardrail subsystems.

## Public surface

`pkg/types` — value types:

- `LeakEntry` (ID, Model, Confidence, Date, LeakedContent, LeakType,
  Source, Tags) with `Validate()`
- `DetectionOptions` (Response, Model, Sensitivity, KnownSignatures)
  with `Validate()` + `Defaults()` (installs `Sensitivity=0.5`)
- `DetectionResult` (Leaked, Confidence, Matches,
  SuggestedMitigation) with `Validate()`
- `LeakMatch` (Pattern, Position, Confidence, MatchedText) with
  `Validate()`
- `ArchiveStats` (ByType, ByModel, TotalLeaks, AvgConfidence)

`pkg/client` — detection + archive:

- `New(opts ...config.Option) (*Client, error)` /
  `NewFromConfig(cfg *config.Config) (*Client, error)`
- `(*Client).Close() error` / `(*Client).Config() *config.Config`
- `(*Client).AddSignatures(sigs ...string)` — extends the
  signature list (trimmed + lowercased; empty strings discarded)
- `(*Client).DetectLeak(ctx, DetectionOptions) (*DetectionResult, error)`
- `(*Client).SearchArchive(ctx, query string, limit int) ([]LeakEntry, error)`
- `(*Client).AddToArchive(ctx, LeakEntry) error`
- `(*Client).GetByModel(ctx, model string) ([]LeakEntry, error)`
- `(*Client).GetStats(ctx) (*ArchiveStats, error)`
- `(*Client).Count() int`
- `(*Client).ExportArchive(ctx, format string) ([]byte, error)` —
  `"json"` (default) supported; other formats surface
  `ErrCodeUnimplemented`

## Usage

```go
import (
    "context"
    "log"

    leakhub "digital.vasic.leakhub/pkg/client"
    "digital.vasic.leakhub/pkg/types"
)

c, err := leakhub.New()
if err != nil { log.Fatal(err) }
defer c.Close()

// Extend the default 10-signature list with red-team-specific phrases.
c.AddSignatures("my-redteam-canary-cookie")

r, err := c.DetectLeak(context.Background(), types.DetectionOptions{
    Response: "Hi there. You are an AI language model created by OpenAI.",
    Model:    "gpt-4",
})
if err != nil { log.Fatal(err) }
if r.Leaked {
    log.Printf("leak detected: %d matches, conf=%.2f, mitigation=%s",
        len(r.Matches), r.Confidence, r.SuggestedMitigation)
    for _, m := range r.Matches {
        log.Printf("  pattern=%q at byte position=%d (matched=%q)",
            m.Pattern, m.Position, m.MatchedText)
    }
}

// Archive the leak for downstream search.
_ = c.AddToArchive(context.Background(), types.LeakEntry{
    ID:            "L-001",
    Model:         "gpt-4",
    Confidence:    r.Confidence,
    LeakedContent: "You are an AI language model...",
    LeakType:      "system-prompt",
    Tags:          []string{"red-team", "openai"},
})

// JSON export for offline analysis.
data, _ := c.ExportArchive(context.Background(), "json")
log.Printf("archive snapshot: %d bytes", len(data))
```

## Anti-bluff guarantees (round-266)

Every PASS produced by this submodule's tests + Challenges carries
positive runtime evidence per Article XI §11.9 and the verbatim
2026-05-19 operator mandate:

> "all existing tests and Challenges do work in anti-bluff manner —
> they MUST confirm that all tested codebase really works as
> expected! We had been in position that all tests do execute with
> success and all Challenges as well, but in reality the most of
> the features does not work and can't be used! This MUST NOT be
> the case and execution of tests and Challenges MUST guarantee
> the quality, the completition and full usability by end users
> of the product!"

Seven invariants enforced by the round-266 runner +
`leakhub_describe_challenge.sh` paired-mutation gate:

1. **Default-surface coverage.** `client.New` MUST seed the 10
   canonical signatures. The runner probes with an ASCII payload
   containing `"you are an ai language model"`, asserts Leaked=true,
   the canonical pattern appears in `Matches`, and
   `SuggestedMitigation` is non-empty.
2. **No false positives on benign locale text.** Per-locale benign
   responses (Paris/Eiffel description in en; Belgrade/Sava in sr;
   Tokyo in ja; Cairo/Nile in ar; Beijing in zh-CN) MUST yield
   Leaked=false + empty Matches. Confirms the default list does not
   over-trigger on legitimate non-ASCII text.
3. **Byte-exact match position on non-ASCII responses.** Per-locale
   leak responses wrap the canonical ASCII signature inside non-ASCII
   greeting + suffix (e.g. `"こんにちは。You are an AI language model
   OpenAIによって作成されました。"`). The runner asserts the reported
   `Position` is a valid byte offset, the response slice at
   `[Position : Position+len(canonical)]` equals the canonical
   phrase, and `MatchedText` equals that slice. Confidence > 0.5.
4. **AddSignatures stores non-ASCII custom signatures.** Each locale
   registers its own non-ASCII custom signature (Cyrillic, Japanese,
   Arabic, Han, English ASCII) via `AddSignatures`; a matching
   `response_with_custom` payload is detected with the custom
   pattern listed in `Matches` and a valid byte `Position`. Proves
   the signature-storage path preserves rune content.
5. **Archive CRUD round-trip across locales.** Five locale entries
   inserted via `AddToArchive` with non-ASCII `LeakedContent` +
   `Tags`. `Count() == 5`. Each locale's per-locale tag (index 1)
   yields a unique `SearchArchive` hit. `GetByModel("gpt-4")`
   returns all 5. `GetStats` reports `TotalLeaks=5`,
   `ByModel["gpt-4"]=5`, `AvgConfidence` in `[0.5, 1.0]`.
6. **JSON export byte-exact round-trip.** `ExportArchive("json")`
   produces a byte slice that unmarshals back into `[]LeakEntry`
   with every locale's `archive_id` present and `LeakedContent`
   byte-equal to the input. `ExportArchive("xml")` MUST surface
   `ErrCodeUnimplemented`.
7. **Paired mutation.** Running the describe gate with
   `--anti-bluff-mutate` plants a deliberate symbol-rename in a
   tmp copy of `docs/test-coverage.md`
   (`DetectLeak -> DetectLeak_MUTATED`), reruns the structural
   cross-reference check, and asserts the gate exits 99. Proves the
   ledger-to-source map actually catches drift instead of
   rubber-stamping it.

A Section that returns success without producing the corresponding
PASS line is a §11.9 violation regardless of how green the summary
line looks.

## Test bank

```bash
# Unit tests (CONST-050(A) — mocks allowed only here)
GOMAXPROCS=2 nice -n 19 go test -count=1 -race -v ./...

# Round-266 challenge runner (real client, 5 locales)
go run ./challenges/runner/ -fixtures tests/fixtures/leakhub/payloads.json

# Describe challenge — clean mode (exit 0)
bash challenges/scripts/leakhub_describe_challenge.sh

# Paired-mutation gate (must exit 99)
bash challenges/scripts/leakhub_describe_challenge.sh --anti-bluff-mutate

# Inherited governance challenges
bash challenges/scripts/no_suspend_calls_challenge.sh
bash challenges/scripts/host_no_auto_suspend_challenge.sh
bash challenges/scripts/chaos_failure_injection_challenge.sh
bash challenges/scripts/ddos_health_flood_challenge.sh
bash challenges/scripts/scaling_horizontal_challenge.sh
bash challenges/scripts/stress_sustained_load_challenge.sh
bash challenges/scripts/ui_terminal_interaction_challenge.sh
bash challenges/scripts/ux_end_to_end_flow_challenge.sh
```

The round-266 runner exits non-zero on any FAIL; the symbol-to-test
ledger lives in `docs/test-coverage.md`.

## Module path & development layout

```go
import "digital.vasic.leakhub"
```

`go.mod` declares the module as `digital.vasic.leakhub` and uses a
relative `replace` directive pointing at `../PliniusCommon`. The
challenge runner `challenges/runner/main.go` lives under the same
module — `go build ./challenges/runner/` from the repo root is
sufficient to produce the runner binary at `/tmp/`.

## Lineage

Extracted from internal HelixAgent research tree on 2026-04-21,
graduated to functional status alongside its 7 sibling Plinius
modules. Round-266 (2026-05-19) adds the deep-doc ledger, the
multi-locale challenge runner, and the paired-mutation describe gate.

Historical research corpus (unused) remains at
`docs/research/go-elder-plinius-v3/go-elder-plinius/go-leakhub/`
inside the HelixAgent repository.

## Governance

This submodule inherits the constitution submodule's universal
rules. See `CLAUDE.md`, `AGENTS.md`, `CONSTITUTION.md` for the
cascaded clauses (CONST-033, CONST-035, CONST-036, CONST-042,
CONST-043, CONST-047..061).

## License

Apache-2.0
