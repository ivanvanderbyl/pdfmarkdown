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

func TestDebug_SOA_FirstPage(t *testing.T) {
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
	require.NoError(t, err)
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	// Load first page
	pageResp, err := instance.FPDF_LoadPage(&requests.FPDF_LoadPage{
		Document: doc.Document,
		Index:    0,
	})
	require.NoError(t, err)
	defer instance.FPDF_ClosePage(&requests.FPDF_ClosePage{
		Page: pageResp.Page,
	})

	config := pdfmarkdown.DefaultConfig()
	page, err := pdfmarkdown.ExtractPage(instance, pageResp.Page, 1, config)
	require.NoError(t, err)

	fmt.Printf("\n=== First Page Analysis ===\n\n")
	fmt.Printf("Total paragraphs: %d\n\n", len(page.Paragraphs))

	// Show first 10 paragraphs
	for pIdx, para := range page.Paragraphs {
		if pIdx >= 10 {
			break
		}

		text := para.Text()
		displayText := text
		if len(displayText) > 80 {
			displayText = displayText[:80] + "..."
		}

		// Get max font size for paragraph
		var maxFontSize float64
		var isBold bool
		for _, line := range para.Lines {
			for _, word := range line.Words {
				if word.FontSize > maxFontSize {
					maxFontSize = word.FontSize
				}
				if word.IsBold {
					isBold = true
				}
			}
		}

		fmt.Printf("Para %d: %.2fpt %s IsHeading=%v HeadingLevel=%d (%d lines)\n",
			pIdx+1, maxFontSize,
			map[bool]string{true: "BOLD", false: ""}[isBold],
			para.IsHeading, para.HeadingLevel, len(para.Lines))
		fmt.Printf("  Text: %q\n\n", displayText)
	}
}
