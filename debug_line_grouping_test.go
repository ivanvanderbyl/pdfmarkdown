package pdfmarkdown

import (
	"fmt"
	"math"
)

// DebugGroupWordsIntoLines is a debug version that logs its decisions
func DebugGroupWordsIntoLines(words []EnrichedWord, debugLog func(string, ...interface{})) []Line {
	if len(words) == 0 {
		return nil
	}

	var lines []Line
	var currentLine []EnrichedWord
	var lineBox Rect
	var baseline float64
	var xHeight float64

	for i, word := range words {
		if len(currentLine) == 0 {
			// Start new line
			debugLog("Word %d: %q - STARTING NEW LINE", i, word.Text)
			currentLine = []EnrichedWord{word}
			lineBox = word.Box
			baseline = word.Baseline
			xHeight = word.XHeight
		} else {
			// Use VISUAL OVERLAP as primary signal
			overlapY0 := math.Max(lineBox.Y0, word.Box.Y0)
			overlapY1 := math.Min(lineBox.Y1, word.Box.Y1)
			overlapHeight := overlapY1 - overlapY0

			wordHeight := word.Box.Height()
			lineHeight := lineBox.Height()
			minHeight := math.Min(wordHeight, lineHeight)

			visuallyOverlapping := overlapHeight > minHeight*0.3

			// Baseline check
			baselineDiff := math.Abs(word.Baseline - baseline)
			threshold := 0.6 * xHeight
			if threshold == 0 {
				threshold = 5.0
			}
			baselineClose := baselineDiff < threshold

			debugLog("Word %d: %q", i, word.Text)
			debugLog("  lineBox Y: %.2f → %.2f (h:%.2f), word Y: %.2f → %.2f (h:%.2f)",
				lineBox.Y0, lineBox.Y1, lineHeight, word.Box.Y0, word.Box.Y1, wordHeight)
			debugLog("  overlap: %.2f, minHeight: %.2f, ratio: %.2f%%, visuallyOverlapping: %v",
				overlapHeight, minHeight, overlapHeight/minHeight*100, visuallyOverlapping)
			debugLog("  baseline: curr=%.2f, word=%.2f, diff=%.2f, threshold=%.2f, baselineClose: %v",
				baseline, word.Baseline, baselineDiff, threshold, baselineClose)

			if visuallyOverlapping || baselineClose {
				debugLog("  → ADDING TO CURRENT LINE")
				currentLine = append(currentLine, word)
				lineBox.X0 = math.Min(lineBox.X0, word.Box.X0)
				lineBox.Y0 = math.Min(lineBox.Y0, word.Box.Y0)
				lineBox.X1 = math.Max(lineBox.X1, word.Box.X1)
				lineBox.Y1 = math.Max(lineBox.Y1, word.Box.Y1)
				baseline = (baseline*float64(len(currentLine)-1) + word.Baseline) / float64(len(currentLine))
			} else {
				debugLog("  → STARTING NEW LINE")
				lines = append(lines, Line{
					Words:    currentLine,
					Box:      lineBox,
					Baseline: baseline,
				})
				currentLine = []EnrichedWord{word}
				lineBox = word.Box
				baseline = word.Baseline
				xHeight = word.XHeight
			}
		}

		// End of text
		if i == len(words)-1 && len(currentLine) > 0 {
			lines = append(lines, Line{
				Words:    currentLine,
				Box:      lineBox,
				Baseline: baseline,
			})
		}
	}

	return lines
}

// Helper to print debug info
func PrintDebug(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}
