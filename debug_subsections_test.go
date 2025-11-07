package pdfmarkdown_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	pdfmarkdown "github.com/ivanvanderbyl/pdfmarkdown"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"
)

// TestDebug_Subsections analyzes subsection title detection
func TestDebug_Subsections(t *testing.T) {
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

	// Get page count
	pageCountResp, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	require.NoError(t, err)

	t.Logf("PDF has %d pages", pageCountResp.PageCount)

	// Analyze page 2 (index 1) where subsections should be
	pageIndex := 1
	if pageCountResp.PageCount < 2 {
		pageIndex = 0
	}

	pageResp, err := instance.FPDF_LoadPage(&requests.FPDF_LoadPage{
		Document: doc.Document,
		Index:    pageIndex,
	})
	require.NoError(t, err)
	defer instance.FPDF_ClosePage(&requests.FPDF_ClosePage{
		Page: pageResp.Page,
	})

	config := pdfmarkdown.DefaultConfig()
	page, err := pdfmarkdown.ExtractPage(instance, pageResp.Page, pageIndex+1, config)
	require.NoError(t, err)

	t.Logf("\n=== SUBSECTION ANALYSIS ===\n")
	t.Logf("Total paragraphs on page: %d\n", len(page.Paragraphs))

	// Show all paragraphs with "Entity" or "Financial" or "External"
	t.Logf("\n--- All Paragraphs (showing first 50 chars) ---")
	for pi, para := range page.Paragraphs {
		text := para.Text()
		if len(text) > 50 {
			text = text[:50] + "..."
		}

		keywords := []string{"Entity", "Financial", "External", "Structure", "Accounts", "Relationships"}
		hasKeyword := false
		for _, kw := range keywords {
			if strings.Contains(para.Text(), kw) {
				hasKeyword = true
				break
			}
		}

		if hasKeyword {
			t.Logf("Para %d [%d lines, heading:%v]: %q", pi, len(para.Lines), para.IsHeading, text)
		}
	}

	// Look for subsection titles
	subsections := []string{"Entity Structure", "Financial Accounts", "External Relationships"}

	for _, subsection := range subsections {
		t.Logf("\n--- Looking for: %q ---", subsection)

		found := false
		for pi, para := range page.Paragraphs {
			text := para.Text()
			if strings.Contains(text, subsection) {
				found = true
				t.Logf("\nFound in paragraph %d", pi)
				t.Logf("  IsHeading: %v", para.IsHeading)
				t.Logf("  HeadingLevel: %d", para.HeadingLevel)
				t.Logf("  Lines: %d", len(para.Lines))
				t.Logf("  Full text: %q", text)

				// Analyze first line (should contain the subsection title)
				if len(para.Lines) > 0 {
					line := para.Lines[0]
					t.Logf("\n  Line 0 analysis:")
					t.Logf("    Words: %d", len(line.Words))

					for wi, word := range line.Words {
						if wi >= 10 {
							break
						}
						t.Logf("    Word %d: %q", wi, word.Text)
						t.Logf("      FontSize: %.2f", word.FontSize)
						t.Logf("      FontWeight: %d", word.FontWeight)
						t.Logf("      IsBold: %v", word.IsBold)
					}
				}

				// Check if it's a single-line paragraph
				if len(para.Lines) == 1 {
					t.Logf("\n  ✓ Single-line paragraph (eligible for heading)")
				} else {
					t.Logf("\n  ✗ Multi-line paragraph (%d lines - NOT eligible for heading)", len(para.Lines))
					if len(para.Lines) > 1 {
						line1Text := ""
						for _, word := range para.Lines[1].Words {
							line1Text += word.Text + " "
						}
						t.Logf("    Line 1 text: %q", line1Text)
					}
				}

				break
			}
		}

		if !found {
			t.Logf("  ✗ Not found in extracted paragraphs")
		}
	}

	// Show overall font size distribution
	t.Logf("\n=== FONT SIZE DISTRIBUTION ===\n")
	fontSizes := make(map[float64]int)
	for _, para := range page.Paragraphs {
		for _, line := range para.Lines {
			for _, word := range line.Words {
				// Round to nearest 0.5
				rounded := float64(int(word.FontSize*2+0.5)) / 2
				fontSizes[rounded]++
			}
		}
	}

	// Sort and display
	var sizes []float64
	for size := range fontSizes {
		sizes = append(sizes, size)
	}
	// Simple sort
	for i := 0; i < len(sizes)-1; i++ {
		for j := i + 1; j < len(sizes); j++ {
			if sizes[j] > sizes[i] {
				sizes[i], sizes[j] = sizes[j], sizes[i]
			}
		}
	}

	for _, size := range sizes {
		count := fontSizes[size]
		t.Logf("  %.1fpt: %d words", size, count)
	}
}
