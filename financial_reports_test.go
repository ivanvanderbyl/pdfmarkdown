package pdfmarkdown

import (
	"strings"
	"testing"
)

func makeCharsFromText(text string, xStart float64, y0 float64, width float64, gapAfter map[int]float64) []EnrichedChar {
	chars := make([]EnrichedChar, 0, len(text))
	x := xStart
	for i, r := range text {
		chars = append(chars, EnrichedChar{
			Text:       r,
			Box:        Rect{X0: x, Y0: y0, X1: x + width, Y1: y0 + 10},
			FontSize:   10,
			FontWeight: 400,
		})
		x += width
		if extra, ok := gapAfter[i]; ok {
			x += extra
		}
	}
	return chars
}

func makeWord(text string, x0, y0, x1, y1 float64) EnrichedWord {
	word := EnrichedWord{
		Text:      text,
		Box:       Rect{X0: x0, Y0: y0, X1: x1, Y1: y1},
		FontSize:  10,
		FontName:  "Helvetica",
		Baseline:  y1 - 1.5,
		XHeight:   6,
		Rotation:  0,
		FillColor: RGBA{A: 255},
	}
	return word
}

func TestGroupCharsIntoWords_SplitsOnLargeVisualGap(t *testing.T) {
	chars := makeCharsFromText("WealthAccelerator", 10, 10, 5, map[int]float64{
		5: 7, // split between "Wealth" and "Accelerator"
	})

	words := groupCharsIntoWords(chars, map[int]bool{})

	if len(words) != 2 {
		t.Fatalf("expected 2 words, got %d: %#v", len(words), words)
	}
	if words[0].Text != "Wealth" || words[1].Text != "Accelerator" {
		t.Fatalf("unexpected words: %#v", words)
	}
}

func TestBuildParagraphsNoDetection_HealsWrappedWordFragments(t *testing.T) {
	config := DefaultConfig()
	words := []EnrichedWord{
		makeWord("Louise", 10, 10, 45, 20),
		makeWord("Loh", 50, 10, 68, 20),
		makeWord("Supera", 72, 10, 112, 20),
		makeWord("nnuation", 72, 23, 122, 33),
		makeWord("Fund", 126, 23, 150, 33),
	}

	paragraphs := buildParagraphsNoDetection(words, 300, config)
	if len(paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paragraphs))
	}

	got := paragraphs[0].Text()
	want := "Louise Loh Superannuation Fund"
	if got != want {
		t.Fatalf("expected healed text %q, got %q", want, got)
	}
}

func TestOrderParagraphsForPage_UsesColumnOrderingForWideTitleAndMultiColumnProse(t *testing.T) {
	paragraphs := []Paragraph{
		{Box: Rect{X0: 0, Y0: 10, X1: 360, Y1: 25}, Lines: []Line{{Words: []EnrichedWord{{Text: "Title"}}}}},
		{Box: Rect{X0: 10, Y0: 100, X1: 80, Y1: 115}, Lines: []Line{{Words: []EnrichedWord{{Text: "Left1"}}}}},
		{Box: Rect{X0: 10, Y0: 120, X1: 80, Y1: 135}, Lines: []Line{{Words: []EnrichedWord{{Text: "Left2"}}}}},
		{Box: Rect{X0: 130, Y0: 100, X1: 200, Y1: 115}, Lines: []Line{{Words: []EnrichedWord{{Text: "Middle1"}}}}},
		{Box: Rect{X0: 250, Y0: 100, X1: 320, Y1: 115}, Lines: []Line{{Words: []EnrichedWord{{Text: "Right1"}}}}},
	}
	columns := []Column{
		{Box: Rect{X0: 0, Y0: 0, X1: 120, Y1: 400}, Index: 0},
		{Box: Rect{X0: 120, Y0: 0, X1: 240, Y1: 400}, Index: 1},
		{Box: Rect{X0: 240, Y0: 0, X1: 400, Y1: 400}, Index: 2},
	}

	ordered := orderParagraphsForPage(paragraphs, columns, nil, 400)
	got := []string{ordered[0].Text(), ordered[1].Text(), ordered[2].Text(), ordered[3].Text(), ordered[4].Text()}
	want := []string{"Title", "Left1", "Left2", "Middle1", "Right1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected reading order at %d: got %v want %v", i, got, want)
		}
	}
}

func TestOrderParagraphsForPage_BypassesColumnsForTableDominantPages(t *testing.T) {
	paragraphs := []Paragraph{
		{Box: Rect{X0: 0, Y0: 10, X1: 360, Y1: 25}, Lines: []Line{{Words: []EnrichedWord{{Text: "Report"}}}}},
		{Box: Rect{X0: 260, Y0: 90, X1: 330, Y1: 105}, Lines: []Line{{Words: []EnrichedWord{{Text: "RightTop"}}}}},
		{Box: Rect{X0: 10, Y0: 100, X1: 80, Y1: 115}, Lines: []Line{{Words: []EnrichedWord{{Text: "LeftLater"}}}}},
	}
	columns := []Column{
		{Box: Rect{X0: 0, Y0: 0, X1: 120, Y1: 400}, Index: 0},
		{Box: Rect{X0: 120, Y0: 0, X1: 240, Y1: 400}, Index: 1},
		{Box: Rect{X0: 240, Y0: 0, X1: 400, Y1: 400}, Index: 2},
	}
	tables := []Table{
		{BBox: CellBBox{X0: 20, Top: 30, X1: 360, Bottom: 80}, NumRows: 5, NumCols: 11},
		{BBox: CellBBox{X0: 20, Top: 120, X1: 360, Bottom: 180}, NumRows: 6, NumCols: 11},
	}

	ordered := orderParagraphsForPage(paragraphs, columns, tables, 400)
	got := []string{ordered[0].Text(), ordered[1].Text(), ordered[2].Text()}
	want := []string{"Report", "RightTop", "LeftLater"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected table-dominant order at %d: got %v want %v", i, got, want)
		}
	}
}

func TestDocumentToMarkdown_SuppressesParagraphsInsideRenderedTable(t *testing.T) {
	doc := &Document{
		Pages: []Page{
			{
				Number: 1,
				Paragraphs: []Paragraph{
					{
						Box: Rect{X0: 10, Y0: 10, X1: 250, Y1: 40},
						Lines: []Line{{
							Words: []EnrichedWord{
								makeWord("Asset", 10, 10, 40, 20),
								makeWord("Code", 100, 10, 130, 20),
								makeWord("Open", 160, 10, 190, 20),
							},
						}},
					},
					{
						Box: Rect{X0: 10, Y0: 50, X1: 250, Y1: 80},
						Lines: []Line{{
							Words: []EnrichedWord{
								makeWord("Cash", 10, 50, 40, 60),
								makeWord("NET0034AU", 100, 50, 160, 60),
								makeWord("76,000.00", 180, 50, 240, 60),
							},
						}},
					},
					{
						Box: Rect{X0: 10, Y0: 90, X1: 220, Y1: 110},
						Lines: []Line{{
							Words: []EnrichedWord{
								makeWord("Outside", 10, 90, 55, 100),
								makeWord("note", 60, 90, 85, 100),
							},
						}},
					},
				},
				Tables: []Table{
					{
						BBox: CellBBox{X0: 0, Top: 0, X1: 260, Bottom: 85},
						Rows: []TableRow{
							{Cells: []TableCell{{Content: "Asset"}, {Content: "Code"}, {Content: "Open value"}}},
							{Cells: []TableCell{{Content: "Cash"}, {Content: "NET0034AU"}, {Content: "76,000.00"}}},
						},
						NumRows: 2,
						NumCols: 3,
					},
				},
			},
		},
	}

	markdown := doc.ToMarkdown(DefaultConfig())
	if strings.Contains(markdown, "Cash NET0034AU 76,000.00") {
		t.Fatalf("expected overlapping paragraph text to be suppressed, got:\n%s", markdown)
	}
	if !strings.Contains(markdown, "| Asset | Code") || !strings.Contains(markdown, "| Cash  | NET0034AU | 76,000.00") {
		t.Fatalf("expected markdown table to be present, got:\n%s", markdown)
	}
	if !strings.Contains(markdown, "Outside note") {
		t.Fatalf("expected non-overlapping note to be preserved, got:\n%s", markdown)
	}
}

func TestDocumentToMarkdown_SuppressesParagraphsWhenTableCoversRowBand(t *testing.T) {
	doc := &Document{
		Pages: []Page{
			{
				Number: 1,
				Paragraphs: []Paragraph{
					{
						Box: Rect{X0: 10, Y0: 120, X1: 300, Y1: 140},
						Lines: []Line{{
							Words: []EnrichedWord{
								makeWord("Total", 10, 120, 36, 130),
								makeWord("Portfolio", 42, 120, 92, 130),
								makeWord("95,423.01", 210, 120, 270, 130),
							},
						}},
					},
				},
				Tables: []Table{
					{
						BBox: CellBBox{X0: 80, Top: 115, X1: 280, Bottom: 145},
						Rows: []TableRow{
							{Cells: []TableCell{{Content: "Metric"}, {Content: "Value"}}},
							{Cells: []TableCell{{Content: "Total Portfolio"}, {Content: "95,423.01"}}},
						},
						NumRows: 2,
						NumCols: 2,
					},
				},
			},
		},
	}

	markdown := doc.ToMarkdown(DefaultConfig())
	if strings.Contains(markdown, "Total Portfolio 95,423.01") {
		t.Fatalf("expected row-band overlap to suppress prose duplicate, got:\n%s", markdown)
	}
}

func TestDocumentToMarkdown_MergesHeaderOnlyTableWithFollowingSameSchemaTables(t *testing.T) {
	doc := &Document{
		Pages: []Page{
			{
				Number: 1,
				Tables: []Table{
					{
						BBox: CellBBox{X0: 0, Top: 10, X1: 300, Bottom: 20},
						Rows: []TableRow{
							{Cells: []TableCell{{Content: "Asset"}, {Content: "Code"}, {Content: "Open value"}}},
						},
						NumRows: 1,
						NumCols: 3,
					},
					{
						BBox: CellBBox{X0: 0, Top: 30, X1: 300, Bottom: 60},
						Rows: []TableRow{
							{Cells: []TableCell{{Content: "Cash Account"}, {Content: ""}, {Content: "103,930.17"}}},
							{Cells: []TableCell{{Content: "Income Receivable"}, {Content: ""}, {Content: "30,530.15"}}},
						},
						NumRows: 2,
						NumCols: 3,
					},
					{
						BBox: CellBBox{X0: 0, Top: 70, X1: 300, Bottom: 100},
						Rows: []TableRow{
							{Cells: []TableCell{{Content: "Koda AUD Trust"}, {Content: "KODAAUD"}, {Content: "0.00"}}},
						},
						NumRows: 1,
						NumCols: 3,
					},
				},
			},
		},
	}

	markdown := doc.ToMarkdown(DefaultConfig())
	if strings.Count(markdown, "| Asset") != 1 {
		t.Fatalf("expected a single merged table header, got:\n%s", markdown)
	}
	if !strings.Contains(markdown, "Cash Account") || !strings.Contains(markdown, "Income Receivable") || !strings.Contains(markdown, "Koda AUD Trust") {
		t.Fatalf("expected merged table rows to be preserved, got:\n%s", markdown)
	}
	if strings.Contains(markdown, "| Koda AUD Trust | KODAAUD | 0.00 |\n\n  \n|") {
		t.Fatalf("expected merged output without a second standalone table, got:\n%s", markdown)
	}
}

func TestDocumentToMarkdown_SplitsSectionsAndRepeatsHeader(t *testing.T) {
	doc := &Document{
		Pages: []Page{
			{
				Number: 1,
				Paragraphs: []Paragraph{
					{
						Box: Rect{X0: 10, Y0: 25, X1: 260, Y1: 60},
						Lines: []Line{
							{Words: []EnrichedWord{makeWord("Cash", 10, 25, 40, 35)}},
							{Words: []EnrichedWord{makeWord("Cash", 10, 45, 35, 55), makeWord("Account", 40, 45, 85, 55), makeWord("103,930.17", 180, 45, 240, 55)}},
						},
					},
					{
						Box: Rect{X0: 10, Y0: 75, X1: 300, Y1: 120},
						Lines: []Line{
							{Words: []EnrichedWord{makeWord("Australian", 10, 75, 70, 85), makeWord("Equities", 75, 75, 125, 85)}},
							{Words: []EnrichedWord{makeWord("Australian", 10, 95, 70, 105), makeWord("Eagle", 75, 95, 105, 105), makeWord("Trust", 110, 95, 140, 105), makeWord("61,211.99", 220, 95, 280, 105)}},
						},
					},
				},
				Tables: []Table{
					{
						BBox: CellBBox{X0: 0, Top: 10, X1: 300, Bottom: 20},
						Rows: []TableRow{
							{Cells: []TableCell{{Content: "Asset"}, {Content: "Code"}, {Content: "Open value"}}},
						},
						NumRows: 1,
						NumCols: 3,
					},
					{
						BBox: CellBBox{X0: 0, Top: 40, X1: 300, Bottom: 60},
						Rows: []TableRow{
							{Cells: []TableCell{{Content: "Cash Account"}, {Content: ""}, {Content: "103,930.17"}}},
						},
						NumRows: 1,
						NumCols: 3,
					},
					{
						BBox: CellBBox{X0: 0, Top: 90, X1: 300, Bottom: 110},
						Rows: []TableRow{
							{Cells: []TableCell{{Content: "Australian Eagle Trust"}, {Content: "ALR2783AU"}, {Content: "61,211.99"}}},
						},
						NumRows: 1,
						NumCols: 3,
					},
				},
			},
		},
	}

	markdown := doc.ToMarkdown(DefaultConfig())
	if !strings.Contains(markdown, "Cash") || !strings.Contains(markdown, "Australian Equities") {
		t.Fatalf("expected section labels in prose, got:\n%s", markdown)
	}
	if strings.Count(markdown, "| Asset") != 2 {
		t.Fatalf("expected repeated header for each section table, got:\n%s", markdown)
	}
	if strings.Contains(markdown, "Cash Account") && strings.Contains(markdown, "Australian Eagle Trust") && strings.Count(markdown, "| ----- |") == 1 {
		t.Fatalf("expected separate section tables rather than one merged table, got:\n%s", markdown)
	}
}

func TestCreateTable_PreservesLeadingTextColumn(t *testing.T) {
	page := &Page{Number: 1, Width: 320, Height: 200}
	cells := []CellBBox{
		{X0: 100, Top: 10, X1: 170, Bottom: 25},
		{X0: 170, Top: 10, X1: 250, Bottom: 25},
		{X0: 100, Top: 25, X1: 170, Bottom: 40},
		{X0: 170, Top: 25, X1: 250, Bottom: 40},
	}
	words := []EnrichedWord{
		makeWord("Asset", 20, 12, 60, 22),
		makeWord("Code", 115, 12, 145, 22),
		makeWord("Open", 185, 12, 215, 22),
		makeWord("Cash", 20, 27, 48, 37),
		makeWord("AG", 52, 24, 64, 32),
		makeWord("NET0034AU", 108, 27, 162, 37),
		makeWord("76,000.00", 185, 27, 240, 37),
	}

	table := createTable(page, cells, words)
	if table.NumCols != 3 {
		t.Fatalf("expected synthetic leading column to be preserved, got %d cols", table.NumCols)
	}
	if table.Rows[0].Cells[0].Content != "Asset" {
		t.Fatalf("expected header leading cell to contain Asset, got %q", table.Rows[0].Cells[0].Content)
	}
	if table.Rows[1].Cells[0].Content != "Cash AG" {
		t.Fatalf("expected row leading cell to contain asset label and marker, got %q", table.Rows[1].Cells[0].Content)
	}
}

func TestDetectStatementLedgerTable_PreservesExplicitColumns(t *testing.T) {
	page := &Page{
		Number: 1,
		Width:  595,
		Paragraphs: []Paragraph{
			{
				Box: Rect{X0: 10, Y0: 10, X1: 580, Y1: 60},
				Lines: []Line{
					{Words: []EnrichedWord{
						makeWord("Date", 10, 10, 40, 20),
						makeWord("Transaction", 90, 10, 150, 20),
						makeWord("details", 155, 10, 195, 20),
						makeWord("Withdrawals", 320, 10, 390, 20),
						makeWord("Deposits", 420, 10, 470, 20),
						makeWord("Balance", 520, 10, 570, 20),
					}},
					{Words: []EnrichedWord{
						makeWord("28", 10, 25, 22, 35),
						makeWord("Jan", 28, 25, 48, 35),
						makeWord("MB", 90, 25, 105, 35),
						makeWord("Transfer", 110, 25, 155, 35),
						makeWord("Gym", 160, 25, 185, 35),
						makeWord("120.00", 330, 25, 370, 35),
						makeWord("1,212.94", 525, 25, 575, 35),
					}},
					{Words: []EnrichedWord{
						makeWord("29", 10, 40, 22, 50),
						makeWord("Jan", 28, 40, 48, 50),
						makeWord("Opening", 90, 40, 135, 50),
						makeWord("Balance", 140, 40, 185, 50),
						makeWord("450.34", 525, 40, 565, 50),
					}},
				},
			},
			{
				Box: Rect{X0: 20, Y0: 90, X1: 260, Y1: 105},
				Lines: []Line{{Words: []EnrichedWord{
					makeWord("Manage", 20, 90, 60, 100),
					makeWord("your", 65, 90, 90, 100),
					makeWord("money", 95, 90, 130, 100),
				}}},
			},
		},
	}

	table, ok := detectStatementLedgerTable(page)
	if !ok {
		t.Fatal("expected ledger table to be detected")
	}
	if table.NumCols != 5 {
		t.Fatalf("expected 5 explicit columns, got %d", table.NumCols)
	}
	if got := table.Rows[0].Cells[0].Content; got != "Date" {
		t.Fatalf("expected header date column, got %q", got)
	}
	if got := table.Rows[0].Cells[1].Content; got != "Transaction details" {
		t.Fatalf("expected preserved details header, got %q", got)
	}
	if got := table.Rows[2].Cells[1].Content; got != "Opening Balance" {
		t.Fatalf("expected opening balance row to stay in ledger, got %q", got)
	}
}

func TestDetectStatementLedgerTable_MergesContinuationRowsAndExcludesNonLedger(t *testing.T) {
	page := &Page{
		Number: 2,
		Width:  595,
		Paragraphs: []Paragraph{
			{
				Box: Rect{X0: 10, Y0: 10, X1: 580, Y1: 70},
				Lines: []Line{
					{Words: []EnrichedWord{
						makeWord("Date", 10, 10, 40, 20),
						makeWord("Transaction", 90, 10, 150, 20),
						makeWord("details", 155, 10, 195, 20),
						makeWord("Withdrawals", 320, 10, 390, 20),
						makeWord("Deposits", 420, 10, 470, 20),
						makeWord("Balance", 520, 10, 570, 20),
					}},
					{Words: []EnrichedWord{
						makeWord("17", 10, 25, 22, 35),
						makeWord("Feb", 28, 25, 48, 35),
						makeWord("MB", 90, 25, 105, 35),
						makeWord("Transfer", 110, 25, 155, 35),
						makeWord("To", 160, 25, 172, 35),
						makeWord("Dani", 177, 25, 205, 35),
						makeWord("20.00", 330, 25, 365, 35),
						makeWord("676.93", 525, 25, 565, 35),
					}},
					{Words: []EnrichedWord{
						makeWord("Ex", 110, 38, 124, 48),
						makeWord("To", 130, 38, 142, 48),
						makeWord("Bills", 147, 38, 175, 48),
					}},
					{Words: []EnrichedWord{
						makeWord("Carried", 90, 53, 130, 63),
						makeWord("Forward", 135, 53, 180, 63),
						makeWord("626.93", 525, 53, 565, 63),
					}},
				},
			},
			{
				Box: Rect{X0: 20, Y0: 95, X1: 280, Y1: 120},
				Lines: []Line{
					{Words: []EnrichedWord{
						makeWord("Let's", 20, 95, 45, 105),
						makeWord("keep", 50, 95, 75, 105),
						makeWord("scammers", 80, 95, 130, 105),
						makeWord("in", 135, 95, 145, 105),
						makeWord("check", 150, 95, 180, 105),
					}},
				},
			},
		},
	}

	table, ok := detectStatementLedgerTable(page)
	if !ok {
		t.Fatal("expected ledger table to be detected")
	}
	if got := table.Rows[1].Cells[1].Content; got != "MB Transfer To Dani\nEx To Bills" {
		t.Fatalf("expected continuation line to stay in description cell, got %q", got)
	}
	if got := table.Rows[2].Cells[1].Content; got != "Carried Forward" {
		t.Fatalf("expected carried forward row to stay in ledger, got %q", got)
	}
	for _, row := range table.Rows {
		for _, cell := range row.Cells {
			if strings.Contains(cell.Content, "scammers") {
				t.Fatalf("expected non-ledger marketing text to be excluded, got row %#v", row)
			}
		}
	}
}

func TestPreferStatementLedgerTables_ReplacesGenericTablesForStatementPages(t *testing.T) {
	page := &Page{
		Number: 1,
		Width:  595,
		Paragraphs: []Paragraph{
			{
				Box: Rect{X0: 10, Y0: 10, X1: 580, Y1: 60},
				Lines: []Line{
					{Words: []EnrichedWord{
						makeWord("Date", 10, 10, 40, 20),
						makeWord("Transaction", 90, 10, 150, 20),
						makeWord("details", 155, 10, 195, 20),
						makeWord("Withdrawals", 320, 10, 390, 20),
						makeWord("Deposits", 420, 10, 470, 20),
						makeWord("Balance", 520, 10, 570, 20),
					}},
					{Words: []EnrichedWord{
						makeWord("28", 10, 25, 22, 35),
						makeWord("Jan", 28, 25, 48, 35),
						makeWord("Opening", 90, 25, 135, 35),
						makeWord("Balance", 140, 25, 185, 35),
						makeWord("450.34", 525, 25, 565, 35),
					}},
				},
			},
		},
		Tables: []Table{
			{
				BBox: CellBBox{X0: 0, Top: 10, X1: 580, Bottom: 60},
				Rows: []TableRow{
					{Cells: []TableCell{{Content: "28 Jan Opening Balance 450.34"}}},
				},
				NumRows: 1,
				NumCols: 1,
			},
		},
	}

	tables := preferDetectedLedgerTables(page)
	if len(tables) != 1 {
		t.Fatalf("expected detected ledger to replace generic table, got %d tables", len(tables))
	}
	if tables[0].NumCols != 5 {
		t.Fatalf("expected detected ledger column count, got %d", tables[0].NumCols)
	}
	if got := tables[0].Rows[0].Cells[0].Content; got != "Date" {
		t.Fatalf("expected ledger header row to be used, got %q", got)
	}
}

func TestDetectStatementLedgerTable_HandlesDebitWithdrawalHeaderAndLeadingCarriedForward(t *testing.T) {
	page := &Page{
		Number: 2,
		Width:  595,
		Paragraphs: []Paragraph{
			{
				Box: Rect{X0: 10, Y0: 10, X1: 580, Y1: 80},
				Lines: []Line{
					{Words: []EnrichedWord{
						makeWord("Date", 10, 10, 40, 20),
						makeWord("Transaction", 90, 10, 150, 20),
						makeWord("Debit/Withdrawal", 300, 10, 390, 20),
						makeWord("Deposit", 420, 10, 470, 20),
						makeWord("Balance", 520, 10, 570, 20),
					}},
					{Words: []EnrichedWord{
						makeWord("Carried", 90, 25, 130, 35),
						makeWord("Forward", 135, 25, 180, 35),
					}},
					{Words: []EnrichedWord{
						makeWord("17", 10, 40, 22, 50),
						makeWord("Feb", 28, 40, 48, 50),
						makeWord("MB", 90, 40, 105, 50),
						makeWord("Transfer", 110, 40, 155, 50),
						makeWord("To", 160, 40, 172, 50),
						makeWord("Bills", 177, 40, 205, 50),
						makeWord("20.00", 330, 40, 365, 50),
						makeWord("676.93", 525, 40, 565, 50),
					}},
					{Words: []EnrichedWord{
						makeWord("28", 10, 55, 22, 65),
						makeWord("Feb", 28, 55, 48, 65),
						makeWord("Closing", 90, 55, 135, 65),
						makeWord("Balance", 140, 55, 185, 65),
						makeWord("-11,418.20", 320, 55, 380, 65),
						makeWord("11,761.39", 430, 55, 485, 65),
						makeWord("793.53", 525, 55, 565, 65),
					}},
				},
			},
		},
	}

	table, ok := detectStatementLedgerTable(page)
	if !ok {
		t.Fatal("expected ledger table to be detected")
	}
	if table.NumCols != 5 {
		t.Fatalf("expected debit/deposit/balance columns to be preserved, got %d", table.NumCols)
	}
	if got := table.Rows[1].Cells[1].Content; got != "Carried Forward" {
		t.Fatalf("expected carried forward to stay as first ledger row, got %q", got)
	}
	if got := table.Rows[3].Cells[3].Content; got != "11,761.39" {
		t.Fatalf("expected deposit column to be preserved on closing balance row, got %q", got)
	}
}

func TestDetectStatementLedgerTable_KeepsNonAmountReferenceTextOutOfAmountColumns(t *testing.T) {
	page := &Page{
		Number: 1,
		Width:  595,
		Paragraphs: []Paragraph{
			{
				Box: Rect{X0: 10, Y0: 10, X1: 580, Y1: 50},
				Lines: []Line{
					{Words: []EnrichedWord{
						makeWord("Date", 10, 10, 40, 20),
						makeWord("Transaction", 90, 10, 150, 20),
						makeWord("Debit/Withdrawal", 300, 10, 390, 20),
						makeWord("Deposit", 420, 10, 470, 20),
						makeWord("Balance", 520, 10, 570, 20),
					}},
					{Words: []EnrichedWord{
						makeWord("29", 10, 25, 22, 35),
						makeWord("Jan", 28, 25, 48, 35),
						makeWord("Fidelity", 90, 25, 130, 35),
						makeWord("Life", 135, 25, 155, 35),
						makeWord("5012413", 300, 25, 345, 35),
						makeWord("37.40", 360, 25, 395, 35),
						makeWord("412.94", 525, 25, 565, 35),
					}},
				},
			},
		},
	}

	table, ok := detectStatementLedgerTable(page)
	if !ok {
		t.Fatal("expected ledger table to be detected")
	}
	if got := table.Rows[1].Cells[1].Content; got != "Fidelity Life 5012413" {
		t.Fatalf("expected reference text to stay in description column, got %q", got)
	}
	if got := table.Rows[1].Cells[2].Content; got != "37.40" {
		t.Fatalf("expected debit column to contain only amount text, got %q", got)
	}
}

func TestDetectLists_DoesNotMarkDashPrefixedFinancialSubtitleAsBullet(t *testing.T) {
	paragraphs := []Paragraph{
		{
			Box: Rect{X0: 40, Y0: 20, X1: 420, Y1: 34},
			Lines: []Line{
				{
					Words: []EnrichedWord{
						makeWord("-", 40, 20, 46, 30),
						makeWord("Integrated", 54, 20, 110, 30),
						makeWord("(including", 116, 20, 172, 30),
						makeWord("Managed", 178, 20, 226, 30),
						makeWord("Account", 232, 20, 276, 30),
						makeWord("transactions)", 282, 20, 356, 30),
						makeWord(":", 362, 20, 366, 30),
						makeWord("01", 372, 20, 384, 30),
						makeWord("July", 390, 20, 412, 30),
						makeWord("2025", 418, 20, 442, 30),
					},
				},
			},
		},
	}

	detectLists(paragraphs)
	if paragraphs[0].IsList {
		t.Fatalf("expected dash-prefixed financial subtitle not to be marked as list")
	}
}

func TestDocumentToMarkdown_StripsDecorativeLeadingDashFromSubtitle(t *testing.T) {
	doc := &Document{
		Pages: []Page{
			{
				Number: 1,
				Paragraphs: []Paragraph{
					{
						Box: Rect{X0: 40, Y0: 10, X1: 300, Y1: 24},
						IsHeading: true,
						HeadingLevel: 1,
						Lines: []Line{{
							Words: []EnrichedWord{
								makeWord("Cash", 40, 10, 66, 20),
								makeWord("Transaction", 72, 10, 132, 20),
								makeWord("Listing", 138, 10, 178, 20),
							},
						}},
					},
					{
						Box: Rect{X0: 40, Y0: 30, X1: 460, Y1: 44},
						Lines: []Line{{
							Words: []EnrichedWord{
								makeWord("-", 40, 30, 46, 40),
								makeWord("Integrated", 54, 30, 110, 40),
								makeWord("(including", 116, 30, 172, 40),
								makeWord("Managed", 178, 30, 226, 40),
								makeWord("Account", 232, 30, 276, 40),
								makeWord("transactions)", 282, 30, 356, 40),
								makeWord(":", 362, 30, 366, 40),
								makeWord("01", 372, 30, 384, 40),
								makeWord("July", 390, 30, 412, 40),
								makeWord("2025", 418, 30, 442, 40),
							},
						}},
					},
				},
			},
		},
	}

	markdown := doc.ToMarkdown(DefaultConfig())
	if strings.Contains(markdown, "\n- Integrated") {
		t.Fatalf("expected decorative leading dash to be removed from subtitle, got:\n%s", markdown)
	}
	if !strings.Contains(markdown, "Integrated (including Managed Account transactions) : 01 July 2025") {
		t.Fatalf("expected subtitle text to be preserved without leading dash, got:\n%s", markdown)
	}
}

func TestDocumentToMarkdown_RendersLongDecorativeBulletNoteAsParagraph(t *testing.T) {
	doc := &Document{
		Pages: []Page{
			{
				Number: 1,
				Paragraphs: []Paragraph{
					{
						Box: Rect{X0: 40, Y0: 10, X1: 520, Y1: 50},
						IsList: true,
						Lines: []Line{
							{
								Words: []EnrichedWord{
									makeWord("*", 40, 10, 46, 20),
									makeWord("The", 54, 10, 70, 20),
									makeWord("Integrated", 76, 10, 132, 20),
									makeWord("Cash", 138, 10, 162, 20),
									makeWord("Transaction", 168, 10, 228, 20),
									makeWord("Listing", 234, 10, 274, 20),
									makeWord("above", 280, 10, 312, 20),
									makeWord("includes", 318, 10, 362, 20),
									makeWord("all", 368, 10, 382, 20),
									makeWord("cash", 388, 10, 412, 20),
									makeWord("transactions", 418, 10, 486, 20),
									makeWord("for", 492, 10, 506, 20),
									makeWord("the", 40, 24, 54, 34),
									makeWord("period,", 60, 24, 96, 34),
									makeWord("including", 102, 24, 150, 34),
									makeWord("cash", 156, 24, 180, 34),
									makeWord("transactions", 186, 24, 254, 34),
									makeWord("in", 260, 24, 270, 34),
									makeWord("your", 276, 24, 300, 34),
									makeWord("Managed", 306, 24, 354, 34),
									makeWord("Account.", 360, 24, 408, 34),
								},
							},
						},
					},
				},
			},
		},
	}

	markdown := doc.ToMarkdown(DefaultConfig())
	if strings.Contains(markdown, "\n- The Integrated Cash Transaction Listing") {
		t.Fatalf("expected long decorative bullet note to render as paragraph, got:\n%s", markdown)
	}
	if !strings.Contains(markdown, "The Integrated Cash Transaction Listing above includes all cash transactions") {
		t.Fatalf("expected note text to be preserved, got:\n%s", markdown)
	}
}

func TestNormalizeRotatedBlock_RequiresIssue140Signals(t *testing.T) {
	verticalWord := func(text string, x0, y0 float64) EnrichedWord {
		w := makeWord(text, x0, y0, x0+6, y0+6)
		w.Rotation = 270
		return w
	}

	block := TextBlock{
		Rotation: 270,
		Words: []EnrichedWord{
			verticalWord("A", 10, 10),
			verticalWord("B", 10, 18),
			verticalWord("C", 10, 26),
			verticalWord("D", 10, 34),
			verticalWord("E", 10, 42),
			verticalWord("F", 18, 10),
			verticalWord("G", 18, 18),
			verticalWord("H", 18, 26),
		},
	}

	if shouldNormalizeIssue140RotatedBlock(block) {
		t.Fatalf("expected ordinary small rotated block not to trigger issue-140 normalization")
	}
}

func TestRecoverSuspiciousTextBlock_StitchesFragmentedWords(t *testing.T) {
	block := TextBlock{
		Rotation:         0,
		ReadingDirection: "ltr",
		Lines: []Line{
			{
				Words: []EnrichedWord{
					makeWord("Assoc", 10, 10, 42, 20),
					makeWord("iated", 43, 10, 75, 20),
					makeWord("claims", 84, 10, 122, 20),
				},
				Box:      Rect{X0: 10, Y0: 10, X1: 122, Y1: 20},
				Baseline: 18.5,
			},
			{
				Words: []EnrichedWord{
					makeWord("Supp", 10, 26, 38, 36),
					makeWord("orting", 39, 26, 75, 36),
					makeWord("docu", 84, 26, 110, 36),
					makeWord("ments", 111, 26, 143, 36),
				},
				Box:      Rect{X0: 10, Y0: 26, X1: 146, Y1: 36},
				Baseline: 34.5,
			},
		},
	}
	block.Words = append(append([]EnrichedWord(nil), block.Lines[0].Words...), block.Lines[1].Words...)

	recovered, changed := recoverSuspiciousTextBlock(block)
	if !changed {
		t.Fatalf("expected fragmented local block to be recovered")
	}

	got := []string{lineText(recovered.Lines[0]), lineText(recovered.Lines[1])}
	want := []string{"Associated claims", "Supporting documents"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected recovered line %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestRecoverSuspiciousTextBlock_StitchesFragmentedAmounts(t *testing.T) {
	block := TextBlock{
		Rotation:         0,
		ReadingDirection: "ltr",
		Lines: []Line{
			{
				Words: []EnrichedWord{
					makeWord("Total", 10, 10, 40, 20),
					makeWord("$", 46, 10, 50, 20),
					makeWord("253.", 56, 10, 82, 20),
					makeWord("1", 88, 10, 92, 20),
					makeWord("5", 98, 10, 102, 20),
				},
				Box:      Rect{X0: 10, Y0: 10, X1: 102, Y1: 20},
				Baseline: 18.5,
			},
		},
	}
	block.Words = append([]EnrichedWord(nil), block.Lines[0].Words...)

	recovered, changed := recoverSuspiciousTextBlock(block)
	if !changed {
		t.Fatalf("expected fragmented amount block to be recovered")
	}

	got := lineText(recovered.Lines[0])
	want := "Total $253.15"
	if got != want {
		t.Fatalf("unexpected recovered amount line: got %q want %q", got, want)
	}
}

func TestRecoverSuspiciousTextBlock_LeavesHealthyBlockAlone(t *testing.T) {
	block := TextBlock{
		Rotation:         0,
		ReadingDirection: "ltr",
		Lines: []Line{
			{
				Words: []EnrichedWord{
					makeWord("Associated", 10, 10, 70, 20),
					makeWord("claims", 76, 10, 112, 20),
				},
				Box:      Rect{X0: 10, Y0: 10, X1: 112, Y1: 20},
				Baseline: 18.5,
			},
		},
	}
	block.Words = append([]EnrichedWord(nil), block.Lines[0].Words...)

	recovered, changed := recoverSuspiciousTextBlock(block)
	if changed {
		t.Fatalf("expected healthy local block to remain unchanged, got %q", lineText(recovered.Lines[0]))
	}
	if got := lineText(recovered.Lines[0]); got != "Associated claims" {
		t.Fatalf("unexpected healthy line text %q", got)
	}
}

func TestRecoverSuspiciousEncodingBlock_SuppressesLowConfidenceNoise(t *testing.T) {
	block := TextBlock{
		Rotation:         0,
		ReadingDirection: "ltr",
		Lines: []Line{
			{
				Words: []EnrichedWord{
					makeWord("Agaaaaa:", 10, 10, 56, 20),
					makeWord("AAAA", 62, 10, 84, 20),
					makeWord("7728-AA-2076", 90, 10, 160, 20),
					makeWord("AabaaAA", 166, 10, 214, 20),
					makeWord("aambaa6618-647173-54", 220, 10, 340, 20),
				},
				Box:      Rect{X0: 10, Y0: 10, X1: 340, Y1: 20},
				Baseline: 18.5,
			},
		},
	}
	block.Words = append([]EnrichedWord(nil), block.Lines[0].Words...)

	recovered, changed := recoverSuspiciousEncodingBlock(block)
	if !changed {
		t.Fatalf("expected suspicious encoding block to be recovered")
	}

	got := lineText(recovered.Lines[0])
	if got != "7728-AA-2076 aambaa6618-647173-54" {
		t.Fatalf("unexpected recovered encoding line: got %q", got)
	}
}

func TestRecoverSuspiciousEncodingBlock_LeavesHealthyText(t *testing.T) {
	block := TextBlock{
		Rotation:         0,
		ReadingDirection: "ltr",
		Lines: []Line{
			{
				Words: []EnrichedWord{
					makeWord("Approval", 10, 10, 54, 20),
					makeWord("history", 60, 10, 100, 20),
				},
				Box:      Rect{X0: 10, Y0: 10, X1: 100, Y1: 20},
				Baseline: 18.5,
			},
		},
	}
	block.Words = append([]EnrichedWord(nil), block.Lines[0].Words...)

	recovered, changed := recoverSuspiciousEncodingBlock(block)
	if changed {
		t.Fatalf("expected healthy block to remain unchanged, got %q", lineText(recovered.Lines[0]))
	}
}
