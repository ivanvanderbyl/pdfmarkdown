package pdfmarkdown_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	pdfmarkdown "github.com/Alcova-AI/pdfmarkdown"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"
)

// TestDebug_WordOrder shows the actual extracted word order
func TestDebug_WordOrder(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	pdfPath := filepath.Join("testdata", "Mock Statement of Advice.pdf")
	doc, err := instance.OpenDocument(&requests.OpenDocument{
		FilePath: &pdfPath,
	})
	if err != nil {
		t.Skip("Mock Statement of Advice.pdf not found")
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	pageResp, err := instance.FPDF_LoadPage(&requests.FPDF_LoadPage{
		Document: doc.Document,
		Index:    1,
	})
	require.NoError(t, err)
	defer instance.FPDF_ClosePage(&requests.FPDF_ClosePage{
		Page: pageResp.Page,
	})

	config := pdfmarkdown.DefaultConfig()
	page, err := pdfmarkdown.ExtractPage(instance, pageResp.Page, 2, config)
	require.NoError(t, err)

	// Find Entity Structure paragraph
	for pi, para := range page.Paragraphs {
		if strings.Contains(para.Text(), "Entity Structure") {
			t.Logf("\n=== Entity Structure Paragraph (Para %d) ===", pi)
			t.Logf("Lines: %d\n", len(para.Lines))

			// Show each line and its words in detail
			for li, line := range para.Lines {
				t.Logf("\n--- Line %d (baseline: %.2f, Y: %.2f-%.2f) ---", li, line.Baseline, line.Box.Y0, line.Box.Y1)

				lineText := ""
				for _, w := range line.Words {
					lineText += w.Text + " "
				}
				t.Logf("Text: %q", strings.TrimSpace(lineText))
				t.Logf("Words (%d):", len(line.Words))

				for wi, word := range line.Words {
					t.Logf("  [%d] %q: X=(%.2f-%.2f) Y=(%.2f-%.2f) baseline=%.2f",
						wi, word.Text,
						word.Box.X0, word.Box.X1,
						word.Box.Y0, word.Box.Y1,
						word.Baseline)
				}
			}

			break
		}
	}
}
