package pdfmarkdown_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"

	pdfmarkdown "github.com/ivanvanderbyl/pdfmarkdown"
)

func TestConverter_WithoutPageBreaks(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	// Create converter with page breaks disabled
	config := pdfmarkdown.Config{
		IncludePageBreaks:  false,
		MinHeadingFontSize: 1.15,
	}
	converter := pdfmarkdown.NewConverterWithConfig(instance, config)

	// Test with the sample PDF
	samplePath := filepath.Join("..", "riskv2", "testdata", "sample.pdf")
	markdown, err := converter.ConvertFile(samplePath)
	require.NoError(t, err)
	require.NotEmpty(t, markdown)

	// Verify no page breaks
	require.NotContains(t, markdown, "---")

	// Check that the previously split text is now continuous
	// "Invesco S&P 500 Equal" should not be split from "Weight ETF"
	normalized := strings.ReplaceAll(markdown, "\n", " ")
	normalized = strings.ReplaceAll(normalized, "  ", " ")
	// The PDF has "EqualWeight" as one word
	require.Contains(t, normalized, "Invesco S&P 500 EqualWeight ETF")
}

func TestConverter_WithPageBreaks(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	// Use default config (page breaks enabled)
	converter := pdfmarkdown.NewConverter(instance)

	samplePath := filepath.Join("..", "riskv2", "testdata", "sample.pdf")
	markdown, err := converter.ConvertFile(samplePath)
	require.NoError(t, err)
	require.NotEmpty(t, markdown)

	// Verify page breaks are present
	require.Contains(t, markdown, "---")
}

func TestConverter_HeadingDetectionDisabled(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	// Create converter with heading detection disabled
	config := pdfmarkdown.Config{
		IncludePageBreaks:  true,
		MinHeadingFontSize: 0, // Disable heading detection
	}
	converter := pdfmarkdown.NewConverterWithConfig(instance, config)

	samplePath := filepath.Join("..", "riskv2", "testdata", "sample.pdf")
	markdown, err := converter.ConvertFile(samplePath)
	require.NoError(t, err)
	require.NotEmpty(t, markdown)

	// Verify no headings (no lines starting with #)
	lines := strings.SplitSeq(markdown, "\n")
	for line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 {
			require.False(t, strings.HasPrefix(trimmed, "#"), "Found heading: %s", trimmed)
		}
	}
}
