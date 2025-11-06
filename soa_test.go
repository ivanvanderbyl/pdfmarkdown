package pdfmarkdown_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pdfmarkdown "github.com/Alcova-AI/pdfmarkdown"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"
)

func TestConverter_SOA_PDF(t *testing.T) {
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

	// Test with the SOA PDF
	pdfPath := filepath.Join("testdata", "Mock Statement of Advice.pdf")
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Skip("Mock Statement of Advice.pdf not found, skipping test")
		return
	}

	markdown, err := converter.ConvertFile(pdfPath)
	require.NoError(t, err)
	require.NotEmpty(t, markdown)

	// Write output for inspection
	outputPath := filepath.Join("testdata", "soa_output.md")
	os.MkdirAll(filepath.Dir(outputPath), 0755)
	err = os.WriteFile(outputPath, []byte(markdown), 0644)
	require.NoError(t, err)

	t.Logf("Markdown written to: %s\n", outputPath)

	// Check for headings
	lines := strings.Split(markdown, "\n")
	headingCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			headingCount++
			t.Logf("Found heading: %s", trimmed)
		}
	}

	t.Logf("Total headings found: %d", headingCount)

	// Show first 2000 chars
	previewLen := 2000
	if len(markdown) < previewLen {
		previewLen = len(markdown)
	}
	t.Logf("\nFirst %d chars of markdown:\n%s\n", previewLen, markdown[:previewLen])
}
