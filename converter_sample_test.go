package pdfmarkdown_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	pdfmarkdown "github.com/ivanvanderbyl/pdfmarkdown"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"
)

func TestConverter_SamplePDF(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	converter := pdfmarkdown.NewConverter(instance)

	// Test with the sample PDF
	samplePath := filepath.Join("..", "riskv2", "testdata", "sample.pdf")
	if _, err := os.Stat(samplePath); os.IsNotExist(err) {
		t.Skip("Sample PDF not found, skipping test")
		return
	}

	markdown, err := converter.ConvertFile(samplePath)
	require.NoError(t, err)
	require.NotEmpty(t, markdown)

	// Write output for inspection
	outputPath := filepath.Join("testdata", "sample_output.md")
	os.MkdirAll(filepath.Dir(outputPath), 0755)
	err = os.WriteFile(outputPath, []byte(markdown), 0644)
	require.NoError(t, err)

	t.Logf("Markdown written to: %s\n", outputPath)
	t.Logf("Markdown preview (first 500 chars):\n%s\n", markdown[:min(500, len(markdown))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
