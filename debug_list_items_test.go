package pdfmarkdown_test

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pdfmarkdown "github.com/ivanvanderbyl/pdfmarkdown"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"
)

// TestDebug_ListItemSplitting analyzes why list items are split across lines
func TestDebug_ListItemSplitting(t *testing.T) {
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

	// Page 2 has the Entity Structure list
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

	t.Logf("\n=== LIST ITEM SPLITTING ANALYSIS ===\n")

	// Find the Entity Structure paragraph
	for pi, para := range page.Paragraphs {
		text := para.Text()
		if strings.Contains(text, "Entity Structure") {
			t.Logf("Found Entity Structure paragraph (para %d)", pi)
			t.Logf("Total lines: %d", len(para.Lines))
			t.Logf("IsList: %v", para.IsList)
			t.Logf("\n")

			// Analyze each line
			for li, line := range para.Lines {
				lineText := ""
				for _, word := range line.Words {
					lineText += word.Text + " "
				}
				lineText = strings.TrimSpace(lineText)

				t.Logf("Line %d: %q", li, lineText)
				t.Logf("  Words: %d, Baseline: %.2f", len(line.Words), line.Baseline)

				if len(line.Words) > 0 {
					firstWord := line.Words[0]
					t.Logf("  First word: %q", firstWord.Text)
					t.Logf("  First word BBox: Y=(%.2f → %.2f)", firstWord.Box.Y0, firstWord.Box.Y1)
					t.Logf("  Font size: %.2f", firstWord.FontSize)

					if len(line.Words) > 1 {
						lastWord := line.Words[len(line.Words)-1]
						t.Logf("  Last word BBox: Y=(%.2f → %.2f)", lastWord.Box.Y0, lastWord.Box.Y1)
					}
				}

				// Check gap to next line
				if li < len(para.Lines)-1 {
					nextLine := para.Lines[li+1]
					if len(nextLine.Words) > 0 && len(line.Words) > 0 {
						currentBaseline := line.Baseline
						nextBaseline := nextLine.Baseline
						baselineDiff := nextBaseline - currentBaseline

						t.Logf("  Gap to next line: baseline diff = %.2f", baselineDiff)

						// Visual overlap check
						currentLineY0 := line.Box.Y0
						currentLineY1 := line.Box.Y1
						nextLineY0 := nextLine.Box.Y0
						nextLineY1 := nextLine.Box.Y1

						t.Logf("  Current line Y: %.2f → %.2f (height: %.2f)", currentLineY0, currentLineY1, currentLineY1-currentLineY0)
						t.Logf("  Next line Y: %.2f → %.2f (height: %.2f)", nextLineY0, nextLineY1, nextLineY1-nextLineY0)

						// Calculate overlap
						overlapY0 := currentLineY0
						if nextLineY0 > overlapY0 {
							overlapY0 = nextLineY0
						}
						overlapY1 := currentLineY1
						if nextLineY1 < overlapY1 {
							overlapY1 = nextLineY1
						}
						overlapHeight := overlapY1 - overlapY0

						t.Logf("  Y-overlap: %.2f (%.1f%% of min height)", overlapHeight,
							overlapHeight/math.Min(currentLineY1-currentLineY0, nextLineY1-nextLineY0)*100)

						// Check if next line looks like a continuation
						nextLineText := ""
						for _, word := range nextLine.Words {
							nextLineText += word.Text + " "
						}
						nextLineText = strings.TrimSpace(nextLineText)

						if strings.HasPrefix(nextLineText, "(") || strings.HasPrefix(nextLineText, "-") {
							t.Logf("  → Next line starts with '(' or '-', likely a continuation!")
						}
					}
				}

				t.Logf("\n")
			}

			break
		}
	}
}
