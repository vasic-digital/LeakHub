package client

import (
	"context"
	"encoding/json"
	"testing"

	"digital.vasic.leakhub/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetectLeakEmptyResponse — no signatures, benign response.
func TestDetectLeakEmptyResponse(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	res, err := c.DetectLeak(context.Background(), types.DetectionOptions{
		Response: "hello there", Model: "gpt",
	})
	require.NoError(t, err)
	assert.False(t, res.Leaked)
	assert.Empty(t, res.Matches)
}

// TestDetectLeakSignatureMatch — known signature triggers detection.
func TestDetectLeakSignatureMatch(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	res, err := c.DetectLeak(context.Background(), types.DetectionOptions{
		Response: "You are an AI language model designed for help.",
		Model:    "gpt",
	})
	require.NoError(t, err)
	assert.True(t, res.Leaked)
	assert.NotEmpty(t, res.Matches)
	assert.NotEmpty(t, res.SuggestedMitigation)
}

// TestDetectLeakUserSignature — caller-supplied signature appends.
func TestDetectLeakUserSignature(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	res, err := c.DetectLeak(context.Background(), types.DetectionOptions{
		Response:        "totally secret phrase embedded here",
		Model:           "gpt",
		KnownSignatures: []string{"totally secret phrase"},
	})
	require.NoError(t, err)
	assert.True(t, res.Leaked)
}

// TestDetectLeakSensitivityHintFallback — hint words trigger only at high sensitivity.
func TestDetectLeakSensitivityHintFallback(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	// Benign response with "instructions" and high sensitivity.
	res, err := c.DetectLeak(context.Background(), types.DetectionOptions{
		Response:    "read the user's instructions carefully",
		Model:       "gpt",
		Sensitivity: 0.9,
	})
	require.NoError(t, err)
	// Hint path: conf = 0.3 * 0.9 = 0.27; threshold = 1 - 0.9 = 0.1. Leaked=true.
	assert.True(t, res.Leaked)
}

// TestDetectLeakInvalidOptions — empty model rejected.
func TestDetectLeakInvalidOptions(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	_, err = c.DetectLeak(context.Background(), types.DetectionOptions{Response: "x"})
	assert.Error(t, err)
}

// TestAddSignatureTrimAndLowercase — whitespace + case normalised.
func TestAddSignatureTrimAndLowercase(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	c.AddSignatures("  NEW SIG  ", "", "  ")
	res, err := c.DetectLeak(context.Background(), types.DetectionOptions{
		Response: "this contains new sig right here",
		Model:    "gpt",
	})
	require.NoError(t, err)
	assert.True(t, res.Leaked)
}

// TestAddToArchiveValidationFailure — invalid entry rejected.
func TestAddToArchiveValidationFailure(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	err = c.AddToArchive(context.Background(), types.LeakEntry{})
	assert.Error(t, err)
}

// TestStatsAfterNAdds — TotalLeaks == N after N AddToArchive calls.
func TestStatsAfterNAdds(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	for i := 0; i < 5; i++ {
		require.NoError(t, c.AddToArchive(context.Background(), types.LeakEntry{
			ID: "e" + string(rune('0'+i)), Model: "gpt-4",
			LeakType: "system", LeakedContent: "content", Confidence: 0.5,
		}))
	}
	st, err := c.GetStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 5, st.TotalLeaks)
	assert.Equal(t, 5, c.Count())
	assert.InDelta(t, 0.5, st.AvgConfidence, 1e-9)
}

// TestSearchArchiveLimitCapAndEmptyQuery.
func TestSearchArchiveLimitCapAndEmptyQuery(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	for i := 0; i < 4; i++ {
		require.NoError(t, c.AddToArchive(context.Background(), types.LeakEntry{
			ID: "e" + string(rune('0'+i)), Model: "m", LeakType: "t",
			LeakedContent: "system prompt leaked", Confidence: 0.5,
		}))
	}
	all, err := c.SearchArchive(context.Background(), "", 2)
	require.NoError(t, err)
	assert.Len(t, all, 2)
	filtered, err := c.SearchArchive(context.Background(), "nothing-matches", 0)
	require.NoError(t, err)
	assert.Empty(t, filtered)
}

// TestGetByModelFilter.
func TestGetByModelFilter(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	require.NoError(t, c.AddToArchive(context.Background(), types.LeakEntry{
		ID: "a", Model: "gpt-4", LeakType: "t", Confidence: 0.1,
	}))
	require.NoError(t, c.AddToArchive(context.Background(), types.LeakEntry{
		ID: "b", Model: "claude", LeakType: "t", Confidence: 0.1,
	}))
	out, err := c.GetByModel(context.Background(), "claude")
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "b", out[0].ID)
}

// TestExportArchiveJSONRoundTrip — export + unmarshal preserves content.
func TestExportArchiveJSONRoundTrip(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	require.NoError(t, c.AddToArchive(context.Background(), types.LeakEntry{
		ID: "e1", Model: "m", LeakType: "sys", LeakedContent: "x", Confidence: 0.5,
	}))
	data, err := c.ExportArchive(context.Background(), "json")
	require.NoError(t, err)
	var back []types.LeakEntry
	require.NoError(t, json.Unmarshal(data, &back))
	require.Len(t, back, 1)
	assert.Equal(t, "e1", back[0].ID)
}

// TestExportArchiveUnsupportedFormat.
func TestExportArchiveUnsupportedFormat(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	_, err = c.ExportArchive(context.Background(), "yaml")
	assert.Error(t, err)
}
