package pdfmarkdown_test

import (
	"path/filepath"
	"sort"
	"testing"
	"time"

	pdfmarkdown "github.com/Alcova-AI/pdfmarkdown"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"
)

// TestDebug_HeadingCollection mimics the heading detection logic to debug
func TestDebug_HeadingCollection(t *testing.T) {
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

	// Analyze page 2 where subsections are
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

	t.Logf("\n=== SIMULATING HEADING DETECTION LOGIC ===\n")

	// Step 1: Calculate body font size (median)
	var allFontSizes []float64
	for _, para := range page.Paragraphs {
		for _, line := range para.Lines {
			for _, word := range line.Words {
				allFontSizes = append(allFontSizes, word.FontSize)
			}
		}
	}

	sort.Float64s(allFontSizes)
	medianIdx := len(allFontSizes) / 2
	bodyFontSize := allFontSizes[medianIdx]
	t.Logf("Body font size (median): %.2f", bodyFontSize)
	t.Logf("MinHeadingFontSize config: %.2f", config.MinHeadingFontSize)
	t.Logf("Heading threshold: %.2f (body × minHeading)", bodyFontSize*config.MinHeadingFontSize)

	// Step 2: Collect heading font sizes
	fontSizeCount := make(map[float64]int)
	t.Logf("\n--- Collecting Heading Font Sizes ---")

	for pi, para := range page.Paragraphs {
		if len(para.Lines) == 0 || len(para.Lines[0].Words) == 0 {
			continue
		}

		line := para.Lines[0]

		// Get max font size in first line
		var maxFontSize float64
		for _, word := range line.Words {
			if word.FontSize > maxFontSize {
				maxFontSize = word.FontSize
			}
		}

		text := para.Text()
		if len(text) > 40 {
			text = text[:40] + "..."
		}

		if len(para.Lines) > 1 {
			// Multi-line paragraph
			var totalSize float64
			var wordCount int
			for li := 1; li < len(para.Lines); li++ {
				for _, word := range para.Lines[li].Words {
					totalSize += word.FontSize
					wordCount++
				}
			}

			if wordCount > 0 {
				avgRestSize := totalSize / float64(wordCount)
				sizeDiff := maxFontSize - avgRestSize
				sizeRatio := maxFontSize / avgRestSize

				t.Logf("Para %d [%d lines]: %q", pi, len(para.Lines), text)
				t.Logf("  First line size: %.2f, Rest avg: %.2f", maxFontSize, avgRestSize)
				t.Logf("  Ratio: %.2fx, Diff: %.2f", sizeRatio, sizeDiff)

				if maxFontSize >= avgRestSize*1.2 {
					t.Logf("  → Ratio ≥ 1.2x ✓")
				} else {
					t.Logf("  → Ratio < 1.2x ✗")
				}

				if maxFontSize >= bodyFontSize*config.MinHeadingFontSize {
					t.Logf("  → Meets threshold (%.2f ≥ %.2f) ✓", maxFontSize, bodyFontSize*config.MinHeadingFontSize)
					if maxFontSize >= avgRestSize*1.2 {
						fontSizeCount[maxFontSize]++
						t.Logf("  → ADDED to heading sizes")
					}
				} else {
					t.Logf("  → Below threshold ✗")
				}
			}
		} else {
			// Single-line paragraph
			if maxFontSize >= bodyFontSize*config.MinHeadingFontSize {
				fontSizeCount[maxFontSize]++
				t.Logf("Para %d [1 line]: %q → size %.2f ADDED", pi, text, maxFontSize)
			}
		}
	}

	// Step 3: Show collected heading sizes
	t.Logf("\n--- Collected Heading Font Sizes ---")
	for size, count := range fontSizeCount {
		t.Logf("  %.2fpt: %d occurrences", size, count)
	}

	// Step 4: Map to levels
	var headingSizes []float64
	for size := range fontSizeCount {
		headingSizes = append(headingSizes, size)
	}
	sort.Float64s(headingSizes)
	// Reverse
	for i := 0; i < len(headingSizes)/2; i++ {
		j := len(headingSizes) - 1 - i
		headingSizes[i], headingSizes[j] = headingSizes[j], headingSizes[i]
	}

	t.Logf("\n--- Font Size to Heading Level Mapping ---")
	for i, size := range headingSizes {
		if i < 6 {
			t.Logf("  %.2fpt → H%d", size, i+1)
		}
	}
}
