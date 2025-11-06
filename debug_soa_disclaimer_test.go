package pdfmarkdown_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"

	pdfmarkdown "github.com/Alcova-AI/pdfmarkdown"
)

func TestDebug_SOA_DisclaimerText(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	// Load the last page of the PDF
	pdfPath := filepath.Join("testdata", "Mock Statement of Advice.pdf")
	doc, err := instance.OpenDocument(&requests.OpenDocument{
		FilePath: &pdfPath,
	})
	require.NoError(t, err)
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	// Get last page
	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	require.NoError(t, err)

	lastPageIdx := pageCount.PageCount - 1
	pageResp, err := instance.FPDF_LoadPage(&requests.FPDF_LoadPage{
		Document: doc.Document,
		Index:    lastPageIdx,
	})
	require.NoError(t, err)
	defer instance.FPDF_ClosePage(&requests.FPDF_ClosePage{
		Page: pageResp.Page,
	})

	config := pdfmarkdown.DefaultConfig()
	page, err := pdfmarkdown.ExtractPage(instance, pageResp.Page, lastPageIdx+1, config)
	require.NoError(t, err)

	fmt.Printf("\n=== Last Page Analysis (Page %d) ===\n\n", lastPageIdx+1)
	fmt.Printf("Total paragraphs on page: %d\n\n", len(page.Paragraphs))

	// Show all paragraphs
	for pIdx, para := range page.Paragraphs {
		text := para.Text()
		displayText := text
		if len(displayText) > 100 {
			displayText = displayText[:100] + "..."
		}
		fmt.Printf("Para %d (%d lines): %q\n", pIdx+1, len(para.Lines), displayText)

		// Look for "DISCLAIMER" or spaced text
		if len(text) >= 4 && (text[:4] == "DISC" || text[:4] == "D I " || text[:4] == "d i ") {
			fmt.Printf("\n=== FOUND DISCLAIMER (Para %d) ===\n", pIdx+1)
			fmt.Printf("Text length: %d chars\n", len(text))
			fmt.Printf("Full text: %q\n\n", text)

			// Analyze first line words with gaps
			if len(para.Lines) > 0 {
				line := para.Lines[0]
				fmt.Printf("First line has %d words:\n", len(line.Words))
				for wIdx, word := range line.Words {
					if wIdx >= 30 { // Show first 30 words
						fmt.Printf("... (%d more words)\n", len(line.Words)-wIdx)
						break
					}

					gap := 0.0
					if wIdx > 0 {
						prevWord := line.Words[wIdx-1]
						gap = word.Box.X0 - prevWord.Box.X1
					}

					fmt.Printf("  Word %d: %q (%.2fpt, width: %.2f, gap: %.2f, x0: %.2f)\n",
						wIdx+1, word.Text, word.FontSize,
						word.Box.X1-word.Box.X0, gap, word.Box.X0)
				}
			}
			fmt.Printf("\n")
		}
	}
}
