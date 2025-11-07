package pdfmarkdown_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"

	pdfmarkdown "github.com/ivanvanderbyl/pdfmarkdown"
)

func TestDebug_OffMarketPDF_HeadingCandidates(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	// Load the PDF
	pdfPath := filepath.Join("testdata", "Off_market_trade.pdf")
	doc, err := instance.OpenDocument(&requests.OpenDocument{
		FilePath: &pdfPath,
	})
	require.NoError(t, err)
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	// Get page count
	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	require.NoError(t, err)

	fmt.Printf("\n=== Analyzing Large Font Text ===\n\n")

	// Count single-line vs multi-line paragraphs with large fonts
	singleLineCount := 0
	multiLineCount := 0

	// Look for paragraphs with 9pt text
	for i := 0; i < pageCount.PageCount; i++ {
		pageResp, err := instance.FPDF_LoadPage(&requests.FPDF_LoadPage{
			Document: doc.Document,
			Index:    i,
		})
		require.NoError(t, err)

		config := pdfmarkdown.DefaultConfig()
		page, err := pdfmarkdown.ExtractPage(instance, pageResp.Page, i+1, config)
		require.NoError(t, err)

		instance.FPDF_ClosePage(&requests.FPDF_ClosePage{
			Page: pageResp.Page,
		})

		for pIdx, para := range page.Paragraphs {
			// Check if this paragraph has any 9pt text
			has9pt := false
			var maxFontSize float64
			for _, line := range para.Lines {
				for _, word := range line.Words {
					if word.FontSize > maxFontSize {
						maxFontSize = word.FontSize
					}
					if word.FontSize >= 8.5 { // Close to 9pt
						has9pt = true
					}
				}
			}

			if has9pt {
				if len(para.Lines) == 1 {
					singleLineCount++
				} else {
					multiLineCount++
				}

				text := para.Text()
				if len(text) > 100 {
					text = text[:100] + "..."
				}
				lineType := "SINGLE-LINE"
				if len(para.Lines) > 1 {
					lineType = fmt.Sprintf("MULTI-LINE (%d)", len(para.Lines))
				}
				fmt.Printf("Page %d, Para %d: %s, %.2fpt, IsHeading=%v\n",
					i+1, pIdx+1, lineType, maxFontSize, para.IsHeading)
				fmt.Printf("  Text: %q\n\n", text)
			}
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Single-line paragraphs with large font: %d\n", singleLineCount)
	fmt.Printf("Multi-line paragraphs with large font: %d\n", multiLineCount)
}
