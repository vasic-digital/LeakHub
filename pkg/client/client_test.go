package client

import (
	"context"
	"testing"

	"digital.vasic.leakhub/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	client, err := New()
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NoError(t, client.Close())
}

func TestDoubleClose(t *testing.T) {
	client, err := New()
	require.NoError(t, err)
	assert.NoError(t, client.Close())
	assert.NoError(t, client.Close())
}

func TestConfig(t *testing.T) {
	client, err := New()
	require.NoError(t, err)
	defer client.Close()
	assert.NotNil(t, client.Config())
}

func TestDetectLeakHit(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	r, err := c.DetectLeak(context.Background(), types.DetectionOptions{
		Response: "Hi there. You are an AI language model created by OpenAI.",
		Model:    "gpt-4",
	})
	require.NoError(t, err)
	assert.True(t, r.Leaked)
	assert.NotEmpty(t, r.Matches)
	assert.Greater(t, r.Confidence, 0.5)
}

func TestDetectLeakMiss(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	r, err := c.DetectLeak(context.Background(), types.DetectionOptions{
		Response: "The Eiffel Tower is located in Paris, France.",
		Model:    "gpt-4",
	})
	require.NoError(t, err)
	assert.False(t, r.Leaked)
	assert.Empty(t, r.Matches)
}

func TestDetectLeakInvalid(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	_, err = c.DetectLeak(context.Background(), types.DetectionOptions{})
	assert.Error(t, err)
}

func TestAddAndSearchArchive(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	e := types.LeakEntry{
		ID: "L1", Model: "gpt-4", Confidence: 0.9, LeakType: "system-prompt",
		LeakedContent: "You are a helpful assistant.",
		Tags:          []string{"red-team"},
	}
	require.NoError(t, c.AddToArchive(context.Background(), e))
	assert.Equal(t, 1, c.Count())

	res, err := c.SearchArchive(context.Background(), "helpful", 10)
	require.NoError(t, err)
	assert.Len(t, res, 1)

	res2, err := c.SearchArchive(context.Background(), "", 10)
	require.NoError(t, err)
	assert.Len(t, res2, 1)
}

func TestAddToArchiveInvalid(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	err = c.AddToArchive(context.Background(), types.LeakEntry{})
	assert.Error(t, err)
}

func TestGetByModel(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	_ = c.AddToArchive(context.Background(), types.LeakEntry{
		ID: "A", Model: "gpt-4", Confidence: 0.8, LeakType: "x",
	})
	_ = c.AddToArchive(context.Background(), types.LeakEntry{
		ID: "B", Model: "claude-3", Confidence: 0.8, LeakType: "x",
	})
	out, err := c.GetByModel(context.Background(), "gpt-4")
	require.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "A", out[0].ID)
}

func TestGetStats(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	_ = c.AddToArchive(context.Background(), types.LeakEntry{
		ID: "A", Model: "gpt-4", Confidence: 0.9, LeakType: "sp",
	})
	_ = c.AddToArchive(context.Background(), types.LeakEntry{
		ID: "B", Model: "gpt-4", Confidence: 0.5, LeakType: "sp",
	})
	st, err := c.GetStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, st.TotalLeaks)
	assert.InDelta(t, 0.7, st.AvgConfidence, 1e-9)
	assert.Equal(t, 2, st.ByModel["gpt-4"])
}

func TestExportArchive(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	_ = c.AddToArchive(context.Background(), types.LeakEntry{
		ID: "A", Model: "gpt-4", Confidence: 0.9, LeakType: "sp",
	})
	data, err := c.ExportArchive(context.Background(), "json")
	require.NoError(t, err)
	assert.Contains(t, string(data), "\"ID\":\"A\"")

	_, err = c.ExportArchive(context.Background(), "xml")
	assert.Error(t, err)
}

func TestAddSignatures(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	c.AddSignatures("my-secret-cookie")
	r, err := c.DetectLeak(context.Background(), types.DetectionOptions{
		Response: "Here is My-Secret-Cookie value.",
		Model:    "gpt-4",
	})
	require.NoError(t, err)
	assert.True(t, r.Leaked)
}
