package pdfmarkdown_test

import (
	"path/filepath"
	"testing"
	"time"

	pdfmarkdown "github.com/ivanvanderbyl/pdfmarkdown"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"
)

// TestDebug_HeadingSizes analyzes font sizes for heading detection
func TestDebug_HeadingSizes(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	converter := pdfmarkdown.NewConverter(instance)

	pdfPath := filepath.Join("testdata", "Mock Statement of Advice.pdf")
	markdown, err := converter.ConvertFile(pdfPath)
	if err != nil {
		t.Skip("Mock Statement of Advice.pdf not found")
	}
	require.NotEmpty(t, markdown)

	// Extract page to analyze
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

	t.Logf("\n=== HEADING FONT SIZE ANALYSIS ===\n")

	// Find all headings and their font sizes
	type HeadingInfo struct {
		Text      string
		Level     int
		MaxSize   float64
		AvgSize   float64
		AllSizes  []float64
		WordCount int
	}

	var headings []HeadingInfo

	for _, para := range page.Paragraphs {
		if para.IsHeading {
			text := para.Text()
			if len(text) > 60 {
				text = text[:60] + "..."
			}

			// Calculate font size statistics
			var maxSize float64
			var totalSize float64
			var allSizes []float64
			wordCount := 0

			for _, line := range para.Lines {
				for _, word := range line.Words {
					wordCount++
					if word.FontSize > maxSize {
						maxSize = word.FontSize
					}
					totalSize += word.FontSize
					allSizes = append(allSizes, word.FontSize)
				}
			}

			avgSize := 0.0
			if wordCount > 0 {
				avgSize = totalSize / float64(wordCount)
			}

			headings = append(headings, HeadingInfo{
				Text:      text,
				Level:     para.HeadingLevel,
				MaxSize:   maxSize,
				AvgSize:   avgSize,
				AllSizes:  allSizes,
				WordCount: wordCount,
			})
		}
	}

	t.Logf("Found %d headings:\n", len(headings))

	for i, h := range headings {
		t.Logf("\nHeading %d: Level %d", i+1, h.Level)
		t.Logf("  Text: %q", h.Text)
		t.Logf("  Word Count: %d", h.WordCount)
		t.Logf("  Max Font Size: %.2f", h.MaxSize)
		t.Logf("  Avg Font Size: %.2f", h.AvgSize)

		if len(h.AllSizes) > 0 && len(h.AllSizes) <= 10 {
			t.Logf("  All sizes: %v", h.AllSizes)
		} else if len(h.AllSizes) > 10 {
			t.Logf("  Size range: %.2f - %.2f (%d words)", h.AllSizes[0], h.AllSizes[len(h.AllSizes)-1], len(h.AllSizes))
		}
	}

	// Group by heading level
	t.Logf("\n=== HEADINGS BY LEVEL ===\n")
	levelGroups := make(map[int][]HeadingInfo)
	for _, h := range headings {
		levelGroups[h.Level] = append(levelGroups[h.Level], h)
	}

	for level := 1; level <= 6; level++ {
		if hs, ok := levelGroups[level]; ok {
			t.Logf("\nLevel %d (%d headings):", level, len(hs))
			for _, h := range hs {
				t.Logf("  - %q (max: %.2f, avg: %.2f)", h.Text, h.MaxSize, h.AvgSize)
			}
		}
	}
}
