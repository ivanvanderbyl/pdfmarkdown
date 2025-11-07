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

func TestDebug_OffMarketPDF_AllSingleLineParagraphs(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	// Load first page only
	pdfPath := filepath.Join("testdata", "Off_market_trade.pdf")
	doc, err := instance.OpenDocument(&requests.OpenDocument{
		FilePath: &pdfPath,
	})
	require.NoError(t, err)
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

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

	fmt.Printf("\n=== All Paragraphs (Page 1) ===\n\n")

	for pIdx, para := range page.Paragraphs {
		lineType := fmt.Sprintf("%d line", len(para.Lines))
		if len(para.Lines) != 1 {
			lineType = fmt.Sprintf("%d lines", len(para.Lines))
		}

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

		text := para.Text()
		if len(text) > 80 {
			text = text[:80] + "..."
		}

		fmt.Printf("Para %d: %.2fpt %s (%s) - %q\n",
			pIdx+1, maxFontSize,
			map[bool]string{true: "BOLD", false: ""}[isBold], lineType, text)

		// Only show first 50 paragraphs
		if pIdx >= 49 {
			break
		}
	}

	fmt.Printf("\n=== Looking for Step headings ===\n\n")
	stepCount := 0
	for pIdx, para := range page.Paragraphs {
		text := para.Text()
		if len(text) >= 4 && text[:4] == "Step" {
			stepCount++

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

			if len(text) > 80 {
				text = text[:80] + "..."
			}

			fmt.Printf("Para %d: %.2fpt %s (%d lines) IsHeading=%v - %q\n",
				pIdx+1, maxFontSize,
				map[bool]string{true: "BOLD", false: ""}[isBold],
				len(para.Lines), para.IsHeading, text)
		}
	}

	fmt.Printf("\nFound %d Step headings\n", stepCount)
}
