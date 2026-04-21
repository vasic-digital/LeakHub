# LeakHub

Prompt-leak detection, extraction, and archival. Detects potential
system-prompt leaks in AI model responses and maintains a searchable
in-memory archive of known leaks. Part of the Plinius Go service family
used by HelixAgent.

## Status

- Compiles: `go build ./...` exits 0.
- Tests pass under `-race`: 2 packages (types, client), all green.
- Default signature list (10 canonical leak phrases) seeded on `New()`.
  Extend with `AddSignatures`.
- Integration-ready: consumable Go library for the HelixAgent ensemble.

## Purpose

- `pkg/types` — value types: `LeakEntry`, `DetectionOptions`,
  `DetectionResult`, `LeakMatch`, `ArchiveStats`.
- `pkg/client` — leak detection + archive:
  - `DetectLeak(opts)` — match response against signature list
  - `AddSignatures(...string)` — extend the detector
  - `SearchArchive(query, limit)` / `AddToArchive(entry)`
  - `GetByModel(model)` / `GetStats()` / `Count()`
  - `ExportArchive(format)` — JSON export

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

r, err := c.DetectLeak(context.Background(), types.DetectionOptions{
    Response: "You are an AI language model. Be helpful.",
    Model:    "gpt-4",
})
if err != nil { log.Fatal(err) }
if r.Leaked {
    log.Printf("leak detected: %d matches, conf=%.2f", len(r.Matches), r.Confidence)
}
```

## Module path

```go
import "digital.vasic.leakhub"
```

## Lineage

Extracted from internal HelixAgent research tree on 2026-04-21.
Graduated to functional status alongside its 7 sibling Plinius modules.

Historical research corpus (unused) remains at
`docs/research/go-elder-plinius-v3/go-elder-plinius/go-leakhub/` inside
the HelixAgent repository.

## Development layout

This module's `go.mod` declares the module as `digital.vasic.leakhub`
and uses a relative `replace` directive pointing at `../PliniusCommon`.

## License

Apache-2.0
