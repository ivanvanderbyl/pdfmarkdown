package pdfmarkdown_test

import (
	"path/filepath"
	"testing"
	"time"

	pdfmarkdown "github.com/ivanvanderbyl/docmill"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"
)

func TestIssue140_DefaultRenderingProducesReadableTable(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	converter := pdfmarkdown.NewConverterWithConfig(instance, pdfmarkdown.DefaultConfig())
	markdown, err := converter.ConvertFile(filepath.Join("testdata", "issue-140-example.pdf"))
	require.NoError(t, err)

	require.Contains(t, markdown, "UPC code", "expected readable top-table header")
	require.Contains(t, markdown, "Location code", "expected readable top-table header")
	require.Contains(t, markdown, "|         | 0085648100305 | LILYSKMACENTRAL | CHOC ALMND SLTD 40% | 637      | $0.61       | $388.57", "expected readable first product row")
	require.Contains(t, markdown, "|         | 0085648100380 | LILYSKMACENTRAL | SLTD CRMLZD CHC DRK | 688      | $0.61       | $419.68", "expected readable second product row")
	require.Contains(t, markdown, "|         | 0085648100303 | LILYSKMACENTRAL | CHOC DARK 55% ALMND | 560      | $0.61       | $341.60", "expected readable third product row")
	require.Contains(t, markdown, "|         | 0085648100300 | LILYSKMACENTRAL | BAR CHOC DARK 55%   | 415      | $0.61       | $253.15", "expected readable fourth product row")
	require.Contains(t, markdown, "Associated claims", "expected readable section heading")
	require.NotContains(t, markdown, "r e b m u n O P", "expected spaced-letter garble to be removed")
}

// TestIssue140_ImprovedTableDetection tests table detection with proper expectations
// Based on visual analysis of the PDF
//
// The PDF contains a ROTATED purchase order table with the following structure:
//
//	Line no | UPC code     | Location code | Item Description         | Quantity | Bill Amount | Accrued Amount | Handling Rate | PO number
//	--------+--------------+---------------+--------------------------+----------+-------------+----------------+---------------+-----------
//	5       | 0085648100305| LILYSKMACENTRAL| CHOC ALMND SLTD 40%     | 637      | $0.61       | $388.57        | 0.0000        |
//	8       | 0085648100380| LILYSKMACENTRAL| SLTD CRMLZD CHC DRK     | 688      | $0.61       | $419.68        | 0.0000        |
//	...
//
// The PDF is rotated 90 degrees (landscape orientation) which causes:
// - Text coordinates to have negative Y values
// - Words to be concatenated without spaces (PDF rendering artifact)
// - Each row appears as a single "word" in extraction
//
// Current issues:
// 1. Words are merged without spaces: "numberPORateHandling..." instead of "number PO Rate Handling..."
// 2. Rotation causes negative coordinates
// 3. Table structure is detected but cell separation is incorrect
func TestIssue140_ImprovedTableDetection(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	pdfPath := filepath.Join("testdata", "issue-140-example.pdf")
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

	// Use segment-based detection which handles rotated tables better
	config := pdfmarkdown.DefaultConfig()
	config.DetectTables = true
	config.UseSegmentBasedTables = true

	page, err := pdfmarkdown.ExtractPage(instance, pageResp.Page, 1, config)
	require.NoError(t, err)

	t.Logf("Detected %d tables", len(page.Tables))

	require.NotEmpty(t, page.Tables, "expected recovered purchase order table")

	table := page.Tables[0]
	t.Logf("Table dimensions: %d rows × %d columns", table.NumRows, table.NumCols)

	mdDoc := &pdfmarkdown.Document{
		Pages: []pdfmarkdown.Page{*page},
	}
	markdown := mdDoc.ToMarkdown(config)
	t.Logf("\n=== Table in Markdown ===\n%s", markdown)

	require.Equal(t, 9, table.NumCols, "expected recovered 9-column purchase order table")
	require.GreaterOrEqual(t, table.NumRows, 5, "expected header plus four product rows")
	require.Contains(t, markdown, "UPC code")
	require.Contains(t, markdown, "Location code")
	require.Contains(t, markdown, "0085648100305")
	require.Contains(t, markdown, "0085648100380")
	require.Contains(t, markdown, "0085648100303")
	require.Contains(t, markdown, "0085648100300")
	require.Contains(t, markdown, "CHOC ALMND SLTD 40%")
	require.Contains(t, markdown, "SLTD CRMLZD CHC DRK")
	require.Contains(t, markdown, "CHOC DARK 55% ALMND")
	require.Contains(t, markdown, "BAR CHOC DARK 55%")
	require.NotContains(t, markdown, "r e b m u n O P", "expected readable normalized content")
}

// TestIssue140_ExpectedStructure documents the ideal table structure
// This test DOCUMENTS what the table SHOULD look like after fixing rotation/spacing issues
func TestIssue140_ExpectedStructure(t *testing.T) {
	t.Skip("This test documents expected output after rotation/spacing fixes")

	// Expected table structure (9 columns × 5 rows):
	//
	// | Line no | UPC code      | Location code    | Item Description          | Quantity | Bill Amount | Accrued Amount | Handling Rate | PO number |
	// |---------|---------------|------------------|---------------------------|----------|-------------|----------------|---------------|-----------|
	// | 5       | 0085648100305 | LILYSKMACENTRAL  | CHOC ALMND SLTD 40%      | 637      | $0.61       | $388.57        | 0.0000        | [empty]   |
	// | 8       | 0085648100380 | LILYSKMACENTRAL  | SLTD CRMLZD CHC DRK      | 688      | $0.61       | $419.68        | 0.0000        | [empty]   |
	// | 3       | 0085648100303 | LILYSKMACENTRAL  | CHOC DARK 55% ALMND      | 560      | $0.61       | $341.60        | 0.0000        | [empty]   |
	// | 0       | 0085648100300 | LILYSKMACENTRAL  | BAR CHOC DARK 55%        | 415      | $0.61       | $253.15        | 0.0000        | [empty]   |

	expectedTable := map[string]interface{}{
		"columns": []string{
			"Line no",
			"UPC code",
			"Location code",
			"Item Description",
			"Quantity",
			"Bill Amount",
			"Accrued Amount",
			"Handling Rate",
			"PO number",
		},
		"rows": []map[string]string{
			{
				"Line no":          "5",
				"UPC code":         "0085648100305",
				"Location code":    "LILYSKMACENTRAL",
				"Item Description": "CHOC ALMND SLTD 40%",
				"Quantity":         "637",
				"Bill Amount":      "$0.61",
				"Accrued Amount":   "$388.57",
				"Handling Rate":    "0.0000",
				"PO number":        "",
			},
			{
				"Line no":          "8",
				"UPC code":         "0085648100380",
				"Location code":    "LILYSKMACENTRAL",
				"Item Description": "SLTD CRMLZD CHC DRK",
				"Quantity":         "688",
				"Bill Amount":      "$0.61",
				"Accrued Amount":   "$419.68",
				"Handling Rate":    "0.0000",
				"PO number":        "",
			},
			{
				"Line no":          "3",
				"UPC code":         "0085648100303",
				"Location code":    "LILYSKMACENTRAL",
				"Item Description": "CHOC DARK 55% ALMND",
				"Quantity":         "560",
				"Bill Amount":      "$0.61",
				"Accrued Amount":   "$341.60",
				"Handling Rate":    "0.0000",
				"PO number":        "",
			},
			{
				"Line no":          "0",
				"UPC code":         "0085648100300",
				"Location code":    "LILYSKMACENTRAL",
				"Item Description": "BAR CHOC DARK 55%",
				"Quantity":         "415",
				"Bill Amount":      "$0.61",
				"Accrued Amount":   "$253.15",
				"Handling Rate":    "0.0000",
				"PO number":        "",
			},
		},
	}

	t.Logf("Expected table structure:\n%+v", expectedTable)
}

// TestIssue140_KnownLimitations documents current parsing limitations
func TestIssue140_KnownLimitations(t *testing.T) {
	t.Log(`
Known limitations with issue-140-example.pdf:

1. ROTATED TEXT (90 degrees):
   - Text coordinates have negative Y values
   - Current rotation detection may not fully handle this case
   - Rotated text appears as vertical columns instead of horizontal rows

2. WORD BOUNDARY DETECTION:
   - PDF has no whitespace characters between words
   - Words are concatenated: "numberPORateHandling..."
   - mergeCloseWords() may be too aggressive (< 2px gap threshold)
   - Need smarter word splitting based on:
     * Font changes
     * Case changes (camelCase detection)
     * Numeric vs alphabetic boundaries
     * Currency symbols ($)

3. TABLE STRUCTURE:
   - Table is detected but as 1 row × 5 columns instead of 5 rows × 9 columns
   - Rotation causes rows to appear as columns
   - Need rotation-aware table reconstruction

4. POTENTIAL FIXES:
   a) Improve rotation normalization for 90° rotated tables
   b) Add word boundary detection without requiring whitespace:
      - Split on case changes (camelCase, PascalCase)
      - Split before currency symbols
      - Split on numeric/alpha transitions
   c) Post-process concatenated text to insert spaces
   d) Handle rotated tables explicitly in table detection

For now, the test validates that:
- Content is extracted (even if concatenated)
- Tables are detected (even if structure is incorrect)
- Key data values are present (UPC codes, amounts, etc.)

Future improvements should focus on word boundary detection for PDFs
without explicit whitespace characters.
	`)
}
