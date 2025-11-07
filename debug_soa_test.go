package pdfmarkdown_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"

	pdfmarkdown "github.com/Alcova-AI/pdfmarkdown"
)

func TestDebug_SOA_FontAnalysis(t *testing.T) {
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
	pdfPath := filepath.Join("testdata", "Mock Statement of Advice.pdf")
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

	fmt.Printf("\n=== SOA PDF Font Analysis ===\n")
	fmt.Printf("Total pages: %d\n\n", pageCount.PageCount)

	// Collect all font sizes across all pages
	fontSizeCount := make(map[float64]int)
	var allFontSizes []float64
	weightCounts := make(map[int]int)

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

		for _, para := range page.Paragraphs {
			for _, line := range para.Lines {
				for _, word := range line.Words {
					fontSizeCount[word.FontSize]++
					allFontSizes = append(allFontSizes, word.FontSize)
					weightCounts[word.FontWeight]++
				}
			}
		}
	}

	// Calculate statistics
	sort.Float64s(allFontSizes)
	medianIdx := len(allFontSizes) / 2
	median := allFontSizes[medianIdx]

	fmt.Printf("Font Size Statistics:\n")
	fmt.Printf("  Median: %.2f\n", median)
	fmt.Printf("  Threshold (1.15x): %.2f\n\n", median*1.15)

	// Sort font sizes by frequency
	type fontSizeFreq struct {
		size  float64
		count int
	}
	var frequencies []fontSizeFreq
	for size, count := range fontSizeCount {
		frequencies = append(frequencies, fontSizeFreq{size, count})
	}
	sort.Slice(frequencies, func(i, j int) bool {
		return frequencies[i].count > frequencies[j].count
	})

	fmt.Printf("Font Size Distribution:\n")
	for i, f := range frequencies {
		if i >= 10 {
			break
		}
		percentage := float64(f.count) / float64(len(allFontSizes)) * 100
		ratio := f.size / median
		isHeading := ""
		if f.size >= median*1.15 {
			isHeading = " [HEADING]"
		}
		fmt.Printf("  %.2f pt: %d words (%.1f%%) - %.2fx%s\n",
			f.size, f.count, percentage, ratio, isHeading)
	}

	// Font weights
	fmt.Printf("\nFont Weights:\n")
	type weightFreq struct {
		weight int
		count  int
	}
	var weights []weightFreq
	for weight, count := range weightCounts {
		weights = append(weights, weightFreq{weight, count})
	}
	sort.Slice(weights, func(i, j int) bool {
		return weights[i].count > weights[j].count
	})
	for _, w := range weights {
		isBold := ""
		if w.weight >= 700 {
			isBold = " [BOLD]"
		}
		percentage := float64(w.count) / float64(len(allFontSizes)) * 100
		fmt.Printf("  Weight %d: %d words (%.1f%%)%s\n", w.weight, w.count, percentage, isBold)
	}
}

func TestDebug_SOA_LineBreaks(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	// Load first page
	pdfPath := filepath.Join("testdata", "Mock Statement of Advice.pdf")
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

	fmt.Printf("\n=== SOA Page 1 - Line Break Analysis ===\n\n")

	// Show first 20 paragraphs
	for pIdx, para := range page.Paragraphs {
		if pIdx >= 20 {
			break
		}

		text := para.Text()
		// Show if text contains newlines
		hasNewlines := strings.Contains(text, "\n")
		newlineCount := strings.Count(text, "\n")

		lineInfo := fmt.Sprintf("%d line", len(para.Lines))
		if len(para.Lines) != 1 {
			lineInfo = fmt.Sprintf("%d lines", len(para.Lines))
		}

		if len(text) > 80 {
			text = text[:80] + "..."
		}

		fmt.Printf("Para %d (%s): HasNewlines=%v (count=%d) IsHeading=%v\n",
			pIdx+1, lineInfo, hasNewlines, newlineCount, para.IsHeading)
		fmt.Printf("  %q\n\n", text)
	}
}

func TestDebug_SOA_Tables(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	// Load all pages
	pdfPath := filepath.Join("testdata", "Mock Statement of Advice.pdf")
	doc, err := instance.OpenDocument(&requests.OpenDocument{
		FilePath: &pdfPath,
	})
	require.NoError(t, err)
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	require.NoError(t, err)

	fmt.Printf("\n=== SOA - Looking for Table-like Content ===\n\n")

	// Look for paragraphs that might be tables (lots of short lines)
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
			// Look for paragraphs with many lines and short text per line
			if len(para.Lines) >= 3 {
				avgLineLength := 0
				for _, line := range para.Lines {
					lineText := ""
					for _, word := range line.Words {
						lineText += word.Text + " "
					}
					avgLineLength += len(lineText)
				}
				avgLineLength /= len(para.Lines)

				// Might be a table if average line length is short
				if avgLineLength < 50 {
					text := para.Text()
					if len(text) > 200 {
						text = text[:200] + "..."
					}

					fmt.Printf("Page %d, Para %d: %d lines, avg len=%d, IsHeading=%v\n",
						i+1, pIdx+1, len(para.Lines), avgLineLength, para.IsHeading)
					fmt.Printf("  %q\n\n", text)
				}
			}
		}
	}
}
