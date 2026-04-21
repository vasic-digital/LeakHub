// Package client provides the Go client for the LeakHub library.
//
// LeakHub detects potential system-prompt leaks in LLM responses and
// maintains an in-memory searchable archive of known leaks. A default
// signature list ships so the detector is immediately useful; callers
// can extend it via `AddSignatures` and AddToArchive / SearchArchive
// manage the corpus.
//
// Basic usage:
//
//	import leakhub "digital.vasic.leakhub/pkg/client"
//
//	c, err := leakhub.New()
//	if err != nil { log.Fatal(err) }
//	defer c.Close()
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"digital.vasic.pliniuscommon/pkg/config"
	"digital.vasic.pliniuscommon/pkg/errors"

	. "digital.vasic.leakhub/pkg/types"
)

// Client is the Go client for LeakHub.
type Client struct {
	cfg    *config.Config
	mu     sync.RWMutex
	closed bool

	archive    map[string]LeakEntry // keyed by ID
	signatures []string
}

var defaultSignatures = []string{
	"you are an ai language model",
	"as an ai developed by",
	"i am a large language model",
	"system prompt:",
	"you are helpful, harmless, and honest",
	"ignore previous instructions",
	"my instructions are",
	"my system prompt",
	"you were created by",
	"assistant's instructions:",
}

// New creates a new LeakHub client seeded with default signatures.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("leakhub", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "leakhub",
			"invalid configuration", err)
	}
	return &Client{
		cfg:        cfg,
		archive:    make(map[string]LeakEntry),
		signatures: append([]string(nil), defaultSignatures...),
	}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "leakhub",
			"invalid configuration", err)
	}
	return &Client{
		cfg:        cfg,
		archive:    make(map[string]LeakEntry),
		signatures: append([]string(nil), defaultSignatures...),
	}, nil
}

// Close gracefully closes the client.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return nil
}

// Config returns the client configuration.
func (c *Client) Config() *config.Config { return c.cfg }

// AddSignatures extends the known-signature list used by DetectLeak.
func (c *Client) AddSignatures(sigs ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range sigs {
		if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
			c.signatures = append(c.signatures, s)
		}
	}
}

// DetectLeak scans a response for leak signatures.
func (c *Client) DetectLeak(ctx context.Context, opts DetectionOptions) (*DetectionResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "leakhub",
			"invalid parameters", err)
	}
	opts.Defaults()

	c.mu.RLock()
	sigs := append([]string(nil), c.signatures...)
	c.mu.RUnlock()
	sigs = append(sigs, opts.KnownSignatures...)

	lower := strings.ToLower(opts.Response)
	matches := []LeakMatch{}
	for _, sig := range sigs {
		sigL := strings.ToLower(sig)
		if idx := strings.Index(lower, sigL); idx >= 0 {
			matches = append(matches, LeakMatch{
				Pattern:     sig,
				Position:    idx,
				Confidence:  0.85,
				MatchedText: opts.Response[idx : idx+len(sigL)],
			})
		}
	}
	conf := 0.0
	for _, m := range matches {
		conf += m.Confidence
	}
	if len(matches) > 0 {
		conf = conf / float64(len(matches))
	}
	// sensitivity adjusts the floor; higher sensitivity = lower detection threshold
	if opts.Sensitivity > 0 && len(matches) == 0 {
		// look for weaker hints (e.g. "instructions")
		hint := []string{"instructions", "system role", "guardrail"}
		for _, h := range hint {
			if strings.Contains(lower, h) {
				conf = 0.3 * opts.Sensitivity
				break
			}
		}
	}
	leaked := len(matches) > 0 || conf >= (1.0-opts.Sensitivity)
	mitigation := ""
	if leaked {
		mitigation = "Apply output filter to strip system-prompt content before returning to user."
	}
	return &DetectionResult{
		Leaked:              leaked,
		Confidence:          conf,
		Matches:             matches,
		SuggestedMitigation: mitigation,
	}, nil
}

// SearchArchive performs substring search over LeakedContent / LeakType / Model / Tags.
func (c *Client) SearchArchive(ctx context.Context, query string, limit int) ([]LeakEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	q := strings.ToLower(query)
	out := []LeakEntry{}
	for _, e := range c.archive {
		if q == "" {
			out = append(out, e)
			continue
		}
		hay := strings.ToLower(e.LeakedContent + " " + e.LeakType + " " + e.Model +
			" " + strings.Join(e.Tags, " "))
		if strings.Contains(hay, q) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// AddToArchive stores a leak entry.
func (c *Client) AddToArchive(ctx context.Context, entry LeakEntry) error {
	if err := entry.Validate(); err != nil {
		return errors.Wrap(errors.ErrCodeInvalidArgument, "leakhub",
			"invalid entry", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.archive[entry.ID] = entry
	return nil
}

// GetByModel returns archive entries for the given model.
func (c *Client) GetByModel(ctx context.Context, model string) ([]LeakEntry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := []LeakEntry{}
	for _, e := range c.archive {
		if strings.EqualFold(e.Model, model) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// GetStats returns archive statistics.
func (c *Client) GetStats(ctx context.Context) (*ArchiveStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	stats := &ArchiveStats{
		ByType:     map[string]int{},
		ByModel:    map[string]int{},
		TotalLeaks: len(c.archive),
	}
	confSum := 0.0
	for _, e := range c.archive {
		stats.ByType[e.LeakType]++
		stats.ByModel[e.Model]++
		confSum += e.Confidence
	}
	if len(c.archive) > 0 {
		stats.AvgConfidence = confSum / float64(len(c.archive))
	}
	return stats, nil
}

// Count returns the number of entries currently in the archive.
func (c *Client) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.archive)
}

// ExportArchive serialises the archive. Supported formats: "json" (default).
func (c *Client) ExportArchive(ctx context.Context, format string) ([]byte, error) {
	c.mu.RLock()
	entries := make([]LeakEntry, 0, len(c.archive))
	for _, e := range c.archive {
		entries = append(entries, e)
	}
	c.mu.RUnlock()
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	switch strings.ToLower(format) {
	case "", "json":
		return json.Marshal(entries)
	default:
		return nil, errors.New(errors.ErrCodeUnimplemented, "leakhub",
			fmt.Sprintf("unsupported export format: %s", format))
	}
}
