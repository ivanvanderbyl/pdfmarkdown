package pdfmarkdown_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"

	pdfmarkdown "github.com/ivanvanderbyl/pdfmarkdown"
)

func TestDebug_OffMarketPDF_FontSizes(t *testing.T) {
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

	fmt.Printf("\n=== Off-Market Trade PDF Font Analysis ===\n")
	fmt.Printf("Total pages: %d\n\n", pageCount.PageCount)

	// Collect all font sizes across all pages
	fontSizeCount := make(map[float64]int)
	var allFontSizes []float64
	weightCounts := make(map[int]int)
	fontNames := make(map[string]int)
	boldCount := 0

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
					fontNames[word.FontName]++
					if word.IsBold {
						boldCount++
					}
				}
			}
		}
	}

	// Calculate statistics
	sort.Float64s(allFontSizes)
	medianIdx := len(allFontSizes) / 2
	median := allFontSizes[medianIdx]

	var sum float64
	for _, size := range allFontSizes {
		sum += size
	}
	mean := sum / float64(len(allFontSizes))

	min := allFontSizes[0]
	max := allFontSizes[len(allFontSizes)-1]

	fmt.Printf("Font Size Statistics:\n")
	fmt.Printf("  Total words: %d\n", len(allFontSizes))
	fmt.Printf("  Min size: %.2f\n", min)
	fmt.Printf("  Max size: %.2f\n", max)
	fmt.Printf("  Mean size: %.2f\n", mean)
	fmt.Printf("  Median size: %.2f\n", median)
	fmt.Printf("\n")

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
		if i >= 15 {
			break
		}
		percentage := float64(f.count) / float64(len(allFontSizes)) * 100
		ratio := f.size / median
		fmt.Printf("  %.2f pt: %d words (%.1f%%) - %.2fx median\n",
			f.size, f.count, percentage, ratio)
	}
	fmt.Printf("\n")

	// Check for heading candidates (>= 1.15x median)
	threshold := median * 1.15
	fmt.Printf("Heading Detection Threshold: %.2f pt (1.15x median)\n", threshold)
	fmt.Printf("Font sizes >= threshold:\n")

	headingCandidates := 0
	for _, f := range frequencies {
		if f.size >= threshold {
			headingCandidates += f.count
			percentage := float64(f.count) / float64(len(allFontSizes)) * 100
			fmt.Printf("  %.2f pt: %d words (%.1f%%)\n", f.size, f.count, percentage)
		}
	}

	if headingCandidates == 0 {
		fmt.Printf("  (none found)\n")
	}
	fmt.Printf("\n")

	// Font weights
	fmt.Printf("Font Weights:\n")
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
		isBold := "(normal)"
		if w.weight >= 700 {
			isBold = "(BOLD)"
		}
		percentage := float64(w.count) / float64(len(allFontSizes)) * 100
		fmt.Printf("  Weight %d: %d words (%.1f%%) %s\n", w.weight, w.count, percentage, isBold)
	}
	fmt.Printf("\n")

	fmt.Printf("Bold words detected: %d\n", boldCount)
	fmt.Printf("\n")

	// Show sample single-line paragraphs from first page
	fmt.Printf("Sample Single-Line Paragraphs (Page 1):\n")
	pageResp, err := instance.FPDF_LoadPage(&requests.FPDF_LoadPage{
		Document: doc.Document,
		Index:    0,
	})
	require.NoError(t, err)

	config := pdfmarkdown.DefaultConfig()
	page, err := pdfmarkdown.ExtractPage(instance, pageResp.Page, 1, config)
	require.NoError(t, err)

	instance.FPDF_ClosePage(&requests.FPDF_ClosePage{
		Page: pageResp.Page,
	})

	sampleCount := 0
	for _, para := range page.Paragraphs {
		if len(para.Lines) == 1 && sampleCount < 10 {
			line := para.Lines[0]
			var maxFontSize float64
			var isBold bool
			for _, word := range line.Words {
				if word.FontSize > maxFontSize {
					maxFontSize = word.FontSize
				}
				if word.IsBold {
					isBold = true
				}
			}
			text := para.Text()
			if len(text) > 80 {
				text = text[:80] + "..."
			}
			fmt.Printf("  %.2f pt (%.2fx) %s: %q\n",
				maxFontSize, maxFontSize/median,
				map[bool]string{true: "BOLD", false: ""}[isBold], text)
			sampleCount++
		}
	}
}
