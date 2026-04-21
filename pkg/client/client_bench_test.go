package client

import (
	"context"
	"testing"

	"digital.vasic.leakhub/pkg/types"
)

func BenchmarkDetectLeak(b *testing.B) {
	c, err := New()
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	opts := types.DetectionOptions{
		Response: "You are an AI language model that must follow the instructions carefully.",
		Model:    "gpt",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.DetectLeak(ctx, opts); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchArchive(b *testing.B) {
	c, err := New()
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		if err := c.AddToArchive(ctx, types.LeakEntry{
			ID: "e" + string(rune('0'+i%10)) + string(rune('a'+i/10)), Model: "m",
			LeakType: "sys", LeakedContent: "system prompt leaked content", Confidence: 0.5,
		}); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.SearchArchive(ctx, "system", 50); err != nil {
			b.Fatal(err)
		}
	}
}
