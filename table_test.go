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

func TestTableDetection_SOA(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	// Load the SOA PDF
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

	// Test each page for tables
	config := pdfmarkdown.DefaultConfig()
	config.DetectTables = true

	tablesFound := 0
	for pageIdx := 0; pageIdx < pageCount.PageCount; pageIdx++ {
		pageResp, err := instance.FPDF_LoadPage(&requests.FPDF_LoadPage{
			Document: doc.Document,
			Index:    pageIdx,
		})
		require.NoError(t, err)

		page, err := pdfmarkdown.ExtractPage(instance, pageResp.Page, pageIdx+1, config)
		require.NoError(t, err)

		instance.FPDF_ClosePage(&requests.FPDF_ClosePage{
			Page: pageResp.Page,
		})

		if len(page.Tables) > 0 {
			fmt.Printf("\n=== Page %d: Found %d table(s) ===\n", pageIdx+1, len(page.Tables))
			for tIdx, table := range page.Tables {
				fmt.Printf("\nTable %d:\n", tIdx+1)
				fmt.Printf("  Dimensions: %d rows x %d cols\n", table.NumRows, table.NumCols)
				fmt.Printf("  BBox: (%.2f, %.2f) -> (%.2f, %.2f)\n",
					table.BBox.X0, table.BBox.Top, table.BBox.X1, table.BBox.Bottom)

				// Show first few rows
				maxRows := 5
				if len(table.Rows) < maxRows {
					maxRows = len(table.Rows)
				}

				for rowIdx := 0; rowIdx < maxRows; rowIdx++ {
					row := table.Rows[rowIdx]
					fmt.Printf("  Row %d: ", rowIdx+1)
					for cellIdx, cell := range row.Cells {
						content := cell.Content
						if len(content) > 20 {
							content = content[:20] + "..."
						}
						if cellIdx > 0 {
							fmt.Print(" | ")
						}
						fmt.Printf("%q", content)
					}
					fmt.Println()
				}

				if len(table.Rows) > maxRows {
					fmt.Printf("  ... (%d more rows)\n", len(table.Rows)-maxRows)
				}
			}
			tablesFound += len(page.Tables)
		}
	}

	fmt.Printf("\n=== Total tables found: %d ===\n", tablesFound)

	// Note: SOA PDF doesn't have explicit line objects and the text layout doesn't
	// form clear table structures that can be auto-detected. This is expected.
	// The bank statement test validates that line-based detection works correctly.
}

func TestTableDetection_SimpleGrid(t *testing.T) {
	// Test with a simple manufactured table structure
	page := &pdfmarkdown.Page{
		Number: 1,
		Width:  612,
		Height: 792,
		Paragraphs: []pdfmarkdown.Paragraph{
			{
				Lines: []pdfmarkdown.Line{
					{
						Words: []pdfmarkdown.EnrichedWord{
							{Text: "Name", Box: pdfmarkdown.Rect{X0: 100, Y0: 100, X1: 150, Y1: 115}},
							{Text: "Age", Box: pdfmarkdown.Rect{X0: 200, Y0: 100, X1: 230, Y1: 115}},
							{Text: "City", Box: pdfmarkdown.Rect{X0: 300, Y0: 100, X1: 340, Y1: 115}},
						},
					},
				},
			},
			{
				Lines: []pdfmarkdown.Line{
					{
						Words: []pdfmarkdown.EnrichedWord{
							{Text: "John", Box: pdfmarkdown.Rect{X0: 100, Y0: 130, X1: 140, Y1: 145}},
							{Text: "25", Box: pdfmarkdown.Rect{X0: 200, Y0: 130, X1: 220, Y1: 145}},
							{Text: "NYC", Box: pdfmarkdown.Rect{X0: 300, Y0: 130, X1: 330, Y1: 145}},
						},
					},
				},
			},
			{
				Lines: []pdfmarkdown.Line{
					{
						Words: []pdfmarkdown.EnrichedWord{
							{Text: "Jane", Box: pdfmarkdown.Rect{X0: 100, Y0: 160, X1: 140, Y1: 175}},
							{Text: "30", Box: pdfmarkdown.Rect{X0: 200, Y0: 160, X1: 220, Y1: 175}},
							{Text: "LA", Box: pdfmarkdown.Rect{X0: 300, Y0: 160, X1: 320, Y1: 175}},
						},
					},
				},
			},
		},
	}

	settings := pdfmarkdown.DefaultTableSettings()
	tables := pdfmarkdown.DetectTables(page, settings)

	require.Greater(t, len(tables), 0, "Expected to detect at least one table")

	table := tables[0]
	fmt.Printf("\nDetected table: %d rows x %d cols\n", table.NumRows, table.NumCols)

	for i, row := range table.Rows {
		fmt.Printf("Row %d: ", i+1)
		for j, cell := range row.Cells {
			if j > 0 {
				fmt.Print(" | ")
			}
			fmt.Printf("%q", cell.Content)
		}
		fmt.Println()
	}

	require.Equal(t, 3, table.NumRows, "Expected 3 rows")
	require.Equal(t, 3, table.NumCols, "Expected 3 columns")
}
