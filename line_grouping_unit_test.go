package pdfmarkdown

import (
	"math"
	"sort"
	"testing"
)

// TestLineGrouping_ListItemScenario tests the exact scenario from SOA PDF
func TestLineGrouping_ListItemScenario(t *testing.T) {
	// Recreate ALL words from Lines 2, 3, 4 from the actual debug output
	words := []EnrichedWord{
		// Line 2: "1. Smith Family Trust"
		{Text: "1.", Box: Rect{X0: 91.20, Y0: 277.17, X1: 98.21, Y1: 285.20}, Baseline: 283.00, FontSize: 14.67, XHeight: 8.0},
		{Text: "Smith", Box: Rect{X0: 108.40, Y0: 277.07, X1: 137.79, Y1: 285.33}, Baseline: 283.13, FontSize: 14.67, XHeight: 8.0},
		{Text: "Family", Box: Rect{X0: 142.40, Y0: 277.20, X1: 176.24, Y1: 287.50}, Baseline: 285.30, FontSize: 14.67, XHeight: 8.0},
		{Text: "Trust", Box: Rect{X0: 179.70, Y0: 277.20, X1: 206.21, Y1: 285.32}, Baseline: 283.12, FontSize: 14.67, XHeight: 8.0},
		// Line 3: "-" (the problematic hyphen far to the right)
		{Text: "-", Box: Rect{X0: 280.62, Y0: 281.82, X1: 283.59, Y1: 282.84}, Baseline: 280.64, FontSize: 14.67, XHeight: 0.6},
		// Line 4: "(SF0005-001) Discretionary trust for income distribution"
		{Text: "(SF0005-001)", Box: Rect{X0: 210.06, Y0: 277.07, X1: 276.82, Y1: 287.50}, Baseline: 285.30, FontSize: 14.67, XHeight: 8.0},
		{Text: "Discretionary", Box: Rect{X0: 287.83, Y0: 277.20, X1: 351.03, Y1: 287.50}, Baseline: 285.30, FontSize: 14.67, XHeight: 8.0},
		{Text: "trust", Box: Rect{X0: 354.37, Y0: 277.39, X1: 375.48, Y1: 285.32}, Baseline: 283.12, FontSize: 14.67, XHeight: 8.0},
		{Text: "for", Box: Rect{X0: 378.71, Y0: 277.07, X1: 391.59, Y1: 285.32}, Baseline: 283.12, FontSize: 14.67, XHeight: 8.0},
		{Text: "income", Box: Rect{X0: 395.21, Y0: 277.20, X1: 429.48, Y1: 285.32}, Baseline: 283.12, FontSize: 14.67, XHeight: 8.0},
		{Text: "distribution", Box: Rect{X0: 433.35, Y0: 277.20, X1: 485.38, Y1: 285.32}, Baseline: 283.12, FontSize: 14.67, XHeight: 8.0},
	}

	t.Logf("=== BEFORE SORTING ===")
	for i, w := range words {
		t.Logf("%d: %q at X=%.2f Y=(%.2f-%.2f)", i, w.Text, w.Box.X0, w.Box.Y0, w.Box.Y1)
	}

	// Apply the same sort used in buildParagraphs
	sort.Slice(words, func(i, j int) bool {
		wordI := words[i]
		wordJ := words[j]

		// Y-overlap check
		overlapY0 := math.Max(wordI.Box.Y0, wordJ.Box.Y0)
		overlapY1 := math.Min(wordI.Box.Y1, wordJ.Box.Y1)
		overlapHeight := overlapY1 - overlapY0
		minHeight := math.Min(wordI.Box.Height(), wordJ.Box.Height())

		// Same visual line - sort by X
		if overlapHeight > minHeight*0.3 {
			return wordI.Box.X0 < wordJ.Box.X0
		}

		// Different lines - sort by Y
		return wordI.Box.Y0 < wordJ.Box.Y0
	})

	t.Logf("\n=== AFTER SORTING ===")
	for i, w := range words {
		t.Logf("%d: %q at X=%.2f Y=(%.2f-%.2f)", i, w.Text, w.Box.X0, w.Box.Y0, w.Box.Y1)
	}

	// Group into lines
	lines := groupWordsIntoLinesBaseline(words)

	t.Logf("\n=== AFTER GROUPING ===")
	t.Logf("Total lines: %d", len(lines))

	for li, line := range lines {
		lineText := ""
		for _, w := range line.Words {
			lineText += w.Text + " "
		}
		t.Logf("Line %d: %q", li, lineText)
	}

	// Expected: All words on ONE line since they all overlap
	if len(lines) != 1 {
		t.Errorf("Expected 1 line, got %d", len(lines))
		t.Logf("ERROR: Words with Y-overlap should be grouped on same line")
	}
}
