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

// TestDebug_RecParagraphs checks paragraph structure in recommendations
func TestDebug_RecParagraphs(t *testing.T) {
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

	// Page 3 likely has recommendations
	pageResp, err := instance.FPDF_LoadPage(&requests.FPDF_LoadPage{
		Document: doc.Document,
		Index:    2,
	})
	require.NoError(t, err)
	defer instance.FPDF_ClosePage(&requests.FPDF_ClosePage{
		Page: pageResp.Page,
	})

	config := pdfmarkdown.DefaultConfig()
	page, err := pdfmarkdown.ExtractPage(instance, pageResp.Page, 3, config)
	require.NoError(t, err)

	t.Logf("\n=== PARAGRAPH STRUCTURE (Page 3) ===\n")
	t.Logf("Total paragraphs: %d\n", len(page.Paragraphs))

	for pi, para := range page.Paragraphs {
		text := para.Text()
		if len(text) > 60 {
			text = text[:60] + "..."
		}

		// Show paragraphs that contain recommendation numbers
		if strings.Contains(para.Text(), "Excess Cash") || strings.Contains(para.Text(), "SMSF Contribution") ||
			strings.Contains(para.Text(), "Trust Distribution") || strings.Contains(para.Text(), "Portfolio Rebalancing") {

			t.Logf("\nPara %d:", pi)
			t.Logf("  Lines: %d", len(para.Lines))
			t.Logf("  IsHeading: %v (level %d)", para.IsHeading, para.HeadingLevel)
			t.Logf("  IsList: %v", para.IsList)
			t.Logf("  Text: %q", text)

			// Show first line details
			if len(para.Lines) > 0 {
				firstLine := para.Lines[0]
				firstText := ""
				for _, w := range firstLine.Words {
					firstText += w.Text + " "
				}
				t.Logf("  First line: %q", strings.TrimSpace(firstText))

				if len(firstLine.Words) > 0 {
					t.Logf("  First word: %q (fontSize: %.2f, bold: %v)",
						firstLine.Words[0].Text,
						firstLine.Words[0].FontSize,
						firstLine.Words[0].IsBold)
				}
			}
		}
	}
}
