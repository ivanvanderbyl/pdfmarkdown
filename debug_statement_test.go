package pdfmarkdown_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pdfmarkdown "github.com/ivanvanderbyl/pdfmarkdown"
	"github.com/klippa-app/go-pdfium/webassembly"
)

// TestDebug_StatementOfAdvice tests table detection on statement PDF
func TestDebug_StatementOfAdvice(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	if err != nil {
		t.Fatal(err)
	}

	converter := pdfmarkdown.NewConverter(instance)

	pdfPath := filepath.Join("testdata", "Mock Statement of Advice.pdf")
	markdown, err := converter.ConvertFile(pdfPath)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Markdown length: %d", len(markdown))

	// Count how many tables were detected
	tableCount := 0
	lines := strings.Split(markdown, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "|") {
			tableCount++
		}
	}

	t.Logf("Table lines detected: %d", tableCount)

	// Write output for inspection
	outputPath := filepath.Join("testdata", "statement_output_debug.md")
	err = os.WriteFile(outputPath, []byte(markdown), 0644)
	if err != nil {
		t.Logf("Warning: could not write debug output: %v", err)
	} else {
		t.Logf("Debug output written to: %s", outputPath)
	}
}
