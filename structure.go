package pdfmarkdown

import (
	"math"
	"sort"
)

// buildParagraphs groups words into lines and paragraphs with rotation and column awareness.
func buildParagraphs(words []EnrichedWord, pageWidth float64, config Config) []Paragraph {
	if len(words) == 0 {
		return nil
	}

	// Detect text rotation and group into blocks
	textBlocks := detectTextRotation(words)

	// If no rotation detected, create single block with all words
	if len(textBlocks) == 0 {
		// Sort words by baseline (Y position), then X position
		sortedWords := make([]EnrichedWord, len(words))
		copy(sortedWords, words)
		sort.Slice(sortedWords, func(i, j int) bool {
			// Use baseline for better line grouping
			baselineDiff := math.Abs(sortedWords[i].Baseline - sortedWords[j].Baseline)
			if baselineDiff < 3 { // Same line threshold
				return sortedWords[i].Box.X0 < sortedWords[j].Box.X0
			}
			return sortedWords[i].Baseline < sortedWords[j].Baseline
		})

		lines := groupWordsIntoLinesBaseline(sortedWords)

		textBlocks = []TextBlock{
			{
				Words:            sortedWords,
				Lines:            lines,
				Rotation:         0,
				ReadingDirection: "ltr",
			},
		}
	}

	// Merge words that are too close together within each line
	for bi := range textBlocks {
		for li := range textBlocks[bi].Lines {
			textBlocks[bi].Lines[li].Words = mergeCloseWords(textBlocks[bi].Lines[li].Words)
		}
	}

	// Collect all lines from all blocks
	var allLines []Line
	for _, block := range textBlocks {
		allLines = append(allLines, block.Lines...)
	}

	// Group lines into paragraphs with adaptive spacing
	paragraphs := groupLinesIntoParagraphsAdaptive(allLines, pageWidth)

	// Detect columns for reading order
	columns := detectColumns(words, pageWidth)

	// Determine reading order with column awareness
	paragraphs = determineReadingOrder(paragraphs, columns)

	// Detect heading levels
	detectHeadings(paragraphs, config)

	// Detect lists
	detectLists(paragraphs)

	// Detect code blocks
	detectCodeBlocks(paragraphs)

	return paragraphs
}

// groupWordsIntoLines groups words that are on the same horizontal line.
func groupWordsIntoLines(words []EnrichedWord) []Line {
	if len(words) == 0 {
		return nil
	}

	var lines []Line
	var currentLine []EnrichedWord
	var lineBox Rect
	var baseline float64

	for i, word := range words {
		wordBaseline := word.Box.Y1 // Bottom of word is the baseline

		if len(currentLine) == 0 {
			// Start new line
			currentLine = []EnrichedWord{word}
			lineBox = word.Box
			baseline = wordBaseline
		} else {
			// Check if word belongs to current line
			yDiff := math.Abs(wordBaseline - baseline)
			if yDiff < 3 { // Same line threshold in points
				// Add to current line
				currentLine = append(currentLine, word)
				lineBox.X0 = math.Min(lineBox.X0, word.Box.X0)
				lineBox.Y0 = math.Min(lineBox.Y0, word.Box.Y0)
				lineBox.X1 = math.Max(lineBox.X1, word.Box.X1)
				lineBox.Y1 = math.Max(lineBox.Y1, word.Box.Y1)
			} else {
				// End current line, start new one
				lines = append(lines, Line{
					Words:    currentLine,
					Box:      lineBox,
					Baseline: baseline,
				})
				currentLine = []EnrichedWord{word}
				lineBox = word.Box
				baseline = wordBaseline
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

// groupWordsIntoLinesBaseline groups words into lines using baseline-aware algorithm
func groupWordsIntoLinesBaseline(words []EnrichedWord) []Line {
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
			currentLine = []EnrichedWord{word}
			lineBox = word.Box
			baseline = word.Baseline
			xHeight = word.XHeight
		} else {
			// Check if word belongs to current line using baseline and x-height
			baselineDiff := math.Abs(word.Baseline - baseline)
			threshold := 0.4 * xHeight // Adaptive threshold based on x-height

			if threshold == 0 {
				threshold = 3.0 // Fallback to fixed threshold
			}

			if baselineDiff < threshold {
				// Add to current line
				currentLine = append(currentLine, word)
				lineBox.X0 = math.Min(lineBox.X0, word.Box.X0)
				lineBox.Y0 = math.Min(lineBox.Y0, word.Box.Y0)
				lineBox.X1 = math.Max(lineBox.X1, word.Box.X1)
				lineBox.Y1 = math.Max(lineBox.Y1, word.Box.Y1)
				// Update baseline to weighted average
				baseline = (baseline*float64(len(currentLine)-1) + word.Baseline) / float64(len(currentLine))
			} else {
				// End current line, start new one
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

// groupLinesIntoParagraphsAdaptive groups lines into paragraphs using adaptive spacing
func groupLinesIntoParagraphsAdaptive(lines []Line, pageWidth float64) []Paragraph {
	if len(lines) == 0 {
		return nil
	}

	// Calculate dynamic threshold based on line spacing distribution
	threshold := calculateDynamicThreshold(lines)

	var paragraphs []Paragraph
	var currentPara []Line
	var paraBox Rect
	var prevLineBottom float64

	for i, line := range lines {
		if len(currentPara) == 0 {
			// Start new paragraph
			currentPara = []Line{line}
			paraBox = line.Box
			prevLineBottom = line.Box.Y1
		} else {
			// Check if line belongs to current paragraph
			lineGap := line.Box.Y0 - prevLineBottom
			avgFontSize := getAverageFontSize(currentPara)
			currentLineFontSize := getLineFontSize(line)

			// Check for significant font size change
			fontSizeRatio := currentLineFontSize / avgFontSize
			significantFontChange := fontSizeRatio < 0.8 || fontSizeRatio > 1.2

			// Use adaptive threshold
			normalizedGap := lineGap / avgFontSize

			if normalizedGap > threshold || significantFontChange {
				// End current paragraph, start new one
				paragraphs = append(paragraphs, Paragraph{
					Lines:     currentPara,
					Box:       paraBox,
					Alignment: detectAlignment(currentPara, pageWidth),
					Indent:    currentPara[0].Box.X0,
				})
				currentPara = []Line{line}
				paraBox = line.Box
			} else {
				// Add to current paragraph
				currentPara = append(currentPara, line)
				paraBox.Y1 = line.Box.Y1
				paraBox.X0 = math.Min(paraBox.X0, line.Box.X0)
				paraBox.X1 = math.Max(paraBox.X1, line.Box.X1)
			}
			prevLineBottom = line.Box.Y1
		}

		// End of text
		if i == len(lines)-1 && len(currentPara) > 0 {
			paragraphs = append(paragraphs, Paragraph{
				Lines:     currentPara,
				Box:       paraBox,
				Alignment: detectAlignment(currentPara, pageWidth),
				Indent:    currentPara[0].Box.X0,
			})
		}
	}

	return paragraphs
}

// calculateDynamicThreshold calculates adaptive paragraph spacing threshold
func calculateDynamicThreshold(lines []Line) float64 {
	if len(lines) < 3 {
		return 0.9 // Fallback to default
	}

	// Calculate all line gaps and font sizes
	var gaps []float64
	var fontSizes []float64

	for i := 0; i < len(lines)-1; i++ {
		gap := lines[i+1].Box.Y0 - lines[i].Box.Y1
		gaps = append(gaps, gap)
		fontSizes = append(fontSizes, getLineFontSize(lines[i]))
	}

	if len(gaps) == 0 {
		return 0.9
	}

	// Calculate median gap and standard deviation
	medianGap := calculateMedian(gaps)
	stdDev := calculateStdDev(gaps)
	medianFontSize := calculateMedian(fontSizes)

	// Paragraph break threshold: median + 1.5 * stdDev, normalized by font size
	if medianFontSize == 0 {
		medianFontSize = 12.0
	}

	threshold := (medianGap + 1.5*stdDev) / medianFontSize

	// Clamp to reasonable bounds (0.6x to 1.5x font size)
	return clamp(threshold, 0.6, 1.5)
}

// groupLinesIntoParagraphs groups lines into paragraphs based on spacing and alignment.
func groupLinesIntoParagraphs(lines []Line, pageWidth float64) []Paragraph {
	if len(lines) == 0 {
		return nil
	}

	var paragraphs []Paragraph
	var currentPara []Line
	var paraBox Rect
	var prevLineBottom float64

	for i, line := range lines {
		if len(currentPara) == 0 {
			// Start new paragraph
			currentPara = []Line{line}
			paraBox = line.Box
			prevLineBottom = line.Box.Y1
		} else {
			// Check if line belongs to current paragraph
			lineGap := line.Box.Y0 - prevLineBottom
			avgFontSize := getAverageFontSize(currentPara)
			currentLineFontSize := getLineFontSize(line)

			// Check for significant font size change
			// A decrease of more than 20% suggests a new paragraph (e.g., title followed by metadata)
			fontSizeRatio := currentLineFontSize / avgFontSize
			significantFontChange := fontSizeRatio < 0.8 || fontSizeRatio > 1.2

			// Large gap indicates new paragraph
			// Typical line spacing is ~0.3-0.4x font size, paragraph breaks are ~1.2x+
			// Use 0.9x as threshold to catch paragraph breaks while avoiding false positives
			// Also start new paragraph if there's a significant font size change
			if lineGap > avgFontSize*0.9 || significantFontChange {
				// End current paragraph, start new one
				paragraphs = append(paragraphs, Paragraph{
					Lines:     currentPara,
					Box:       paraBox,
					Alignment: detectAlignment(currentPara, pageWidth),
					Indent:    currentPara[0].Box.X0,
				})
				currentPara = []Line{line}
				paraBox = line.Box
			} else {
				// Add to current paragraph
				currentPara = append(currentPara, line)
				paraBox.Y1 = line.Box.Y1
				paraBox.X0 = math.Min(paraBox.X0, line.Box.X0)
				paraBox.X1 = math.Max(paraBox.X1, line.Box.X1)
			}
			prevLineBottom = line.Box.Y1
		}

		// End of text
		if i == len(lines)-1 && len(currentPara) > 0 {
			paragraphs = append(paragraphs, Paragraph{
				Lines:     currentPara,
				Box:       paraBox,
				Alignment: detectAlignment(currentPara, pageWidth),
				Indent:    currentPara[0].Box.X0,
			})
		}
	}

	return paragraphs
}

// detectAlignment detects the alignment of lines in a paragraph.
func detectAlignment(lines []Line, pageWidth float64) Alignment {
	if len(lines) == 0 {
		return AlignmentLeft
	}

	// Check if all lines start at similar X positions (left aligned)
	var startPositions []float64
	for _, line := range lines {
		startPositions = append(startPositions, line.Box.X0)
	}

	// Check if centered (lines centered around page center)
	pageCenter := pageWidth / 2
	var centerOffsets []float64
	for _, line := range lines {
		lineCenter := (line.Box.X0 + line.Box.X1) / 2
		centerOffsets = append(centerOffsets, math.Abs(lineCenter-pageCenter))
	}

	avgCenterOffset := average(centerOffsets)
	if avgCenterOffset < 20 { // Within 20 points of center
		return AlignmentCenter
	}

	// Check if right aligned
	var endPositions []float64
	for _, line := range lines {
		endPositions = append(endPositions, line.Box.X1)
	}

	endStdDev := stdDev(endPositions)
	startStdDev := stdDev(startPositions)

	if endStdDev < 5 && endStdDev < startStdDev {
		return AlignmentRight
	}

	return AlignmentLeft
}

// detectHeadings identifies paragraphs that are headings and assigns levels.
func detectHeadings(paragraphs []Paragraph, config Config) {
	if len(paragraphs) == 0 || config.MinHeadingFontSize == 0 {
		return
	}

	// Collect all font sizes and calculate statistics
	var allFontSizes []float64
	for _, para := range paragraphs {
		for _, line := range para.Lines {
			for _, word := range line.Words {
				allFontSizes = append(allFontSizes, word.FontSize)
			}
		}
	}

	if len(allFontSizes) == 0 {
		return
	}

	// Calculate body text font size (using median for robustness)
	sort.Float64s(allFontSizes)
	medianIdx := len(allFontSizes) / 2
	bodyFontSize := allFontSizes[medianIdx]

	// Collect distinct font sizes that are meaningfully larger than body text
	// Consider both single-line paragraphs AND first lines of multi-line paragraphs
	fontSizeCount := make(map[float64]int)
	for _, para := range paragraphs {
		if len(para.Lines) == 0 || len(para.Lines[0].Words) == 0 {
			continue
		}

		line := para.Lines[0]

		// Get the maximum font size in the first line
		var maxFontSize float64
		for _, word := range line.Words {
			if word.FontSize > maxFontSize {
				maxFontSize = word.FontSize
			}
		}

		// For multi-line paragraphs, check if first line is a potential subsection heading
		// (larger than the rest of the paragraph content)
		if len(para.Lines) > 1 {
			// Get average font size of remaining lines
			var totalSize float64
			var wordCount int
			for li := 1; li < len(para.Lines); li++ {
				for _, word := range para.Lines[li].Words {
					totalSize += word.FontSize
					wordCount++
				}
			}

			// Only count first line if it's significantly larger than rest of paragraph
			if wordCount > 0 {
				avgRestSize := totalSize / float64(wordCount)
				// Use 1.15x ratio (15% larger) to catch subsection headings
				// that are subtly larger than body text
				if maxFontSize >= avgRestSize*1.15 && maxFontSize >= bodyFontSize*config.MinHeadingFontSize {
					fontSizeCount[maxFontSize]++
				}
			}
		} else {
			// Single-line paragraph - count if larger than body text
			if maxFontSize >= bodyFontSize*config.MinHeadingFontSize {
				fontSizeCount[maxFontSize]++
			}
		}
	}

	// Sort distinct heading font sizes descending
	var headingSizes []float64
	for size := range fontSizeCount {
		headingSizes = append(headingSizes, size)
	}
	sort.Float64s(headingSizes)
	// Reverse to descending order
	for i := 0; i < len(headingSizes)/2; i++ {
		j := len(headingSizes) - 1 - i
		headingSizes[i], headingSizes[j] = headingSizes[j], headingSizes[i]
	}

	// Map font sizes to heading levels (H1 = largest, up to H6)
	sizeToLevel := make(map[float64]int)
	for i, size := range headingSizes {
		if i < 6 {
			sizeToLevel[size] = i + 1
		}
	}

	// Mark headings in paragraphs
	for i := range paragraphs {
		para := &paragraphs[i]

		if len(para.Lines) == 0 || len(para.Lines[0].Words) == 0 {
			continue
		}

		// For multi-line paragraphs, check if the first line is a subsection heading
		// (larger font than the rest of the paragraph)
		if len(para.Lines) > 1 {
			// Get font size of first line
			var firstLineMaxSize float64
			for _, word := range para.Lines[0].Words {
				if word.FontSize > firstLineMaxSize {
					firstLineMaxSize = word.FontSize
				}
			}

			// Get average font size of remaining lines
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

				// If first line is significantly larger (15%+) and meets heading threshold,
				// treat it as a subsection heading
				if firstLineMaxSize >= avgRestSize*1.15 && firstLineMaxSize >= bodyFontSize*config.MinHeadingFontSize {
					// Check if it's in our known heading sizes
					if level, isHeading := sizeToLevel[firstLineMaxSize]; isHeading {
						para.IsHeading = true
						para.HeadingLevel = level
					}
				}
			}

			// Skip further checks for multi-line paragraphs
			continue
		}

		// Single-line paragraph handling
		line := para.Lines[0]

		// Get maximum font size in line
		var maxFontSize float64
		for _, word := range line.Words {
			if word.FontSize > maxFontSize {
				maxFontSize = word.FontSize
			}
		}

		// Check if this line is a heading based on font size
		if level, isHeading := sizeToLevel[maxFontSize]; isHeading {
			para.IsHeading = true
			para.HeadingLevel = level
		} else {
			// Also check if bold + slightly larger
			isBold := false
			for _, word := range line.Words {
				if word.IsBold {
					isBold = true
					break
				}
			}

			// Bold text that's at least 1.05x body size can be a heading
			if isBold && maxFontSize >= bodyFontSize*1.05 && maxFontSize >= bodyFontSize*config.MinHeadingFontSize {
				para.IsHeading = true
				para.HeadingLevel = 6 // Default to H6 for bold-only headings
			}
		}
	}
}

// detectLists identifies paragraphs that are list items.
func detectLists(paragraphs []Paragraph) {
	for i := range paragraphs {
		para := &paragraphs[i]

		// Check first word of first line
		if len(para.Lines) == 0 || len(para.Lines[0].Words) == 0 {
			continue
		}

		firstWord := para.Lines[0].Words[0]
		if firstWord.IsBulletOrNumber() {
			para.IsList = true
		}
	}
}

// detectCodeBlocks identifies paragraphs that are code blocks.
func detectCodeBlocks(paragraphs []Paragraph) {
	for i := range paragraphs {
		para := &paragraphs[i]

		// Check if most words are monospace
		var monoCount int
		var totalWords int
		for _, line := range para.Lines {
			for _, word := range line.Words {
				totalWords++
				if word.IsMonospace {
					monoCount++
				}
			}
		}

		if totalWords > 0 && float64(monoCount)/float64(totalWords) > 0.8 {
			para.IsCode = true
		}
	}
}

// getAverageFontSize calculates the average font size in a set of lines.
func getAverageFontSize(lines []Line) float64 {
	var total float64
	var count int
	for _, line := range lines {
		for _, word := range line.Words {
			total += word.FontSize
			count++
		}
	}
	if count == 0 {
		return 12 // Default
	}
	return total / float64(count)
}

// getLineFontSize calculates the average font size for a single line.
func getLineFontSize(line Line) float64 {
	var total float64
	var count int
	for _, word := range line.Words {
		total += word.FontSize
		count++
	}
	if count == 0 {
		return 12 // Default
	}
	return total / float64(count)
}

// average calculates the average of a slice of floats.
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// stdDev calculates the standard deviation of a slice of floats.
func stdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	avg := average(values)
	var sumSquares float64
	for _, v := range values {
		diff := v - avg
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(values)))
}

// mergeCloseWords merges words that are very close together horizontally.
// This handles PDFs with inconsistent spacing where words are split incorrectly.
// Words with gaps < 2.0 pixels are merged together (except punctuation).
func mergeCloseWords(words []EnrichedWord) []EnrichedWord {
	if len(words) <= 1 {
		return words
	}

	const gapThreshold = 2.0 // pixels

	var merged []EnrichedWord
	var currentMerge []EnrichedWord

	for i, word := range words {
		if len(currentMerge) == 0 {
			currentMerge = []EnrichedWord{word}
			continue
		}

		// Calculate gap from previous word
		prevWord := currentMerge[len(currentMerge)-1]
		gap := word.Box.X0 - prevWord.Box.X1

		// Check if current word is punctuation that should stay separate
		isPunctuation := false
		if len(word.Text) == 1 {
			r := []rune(word.Text)[0]
			isPunctuation = r == '.' || r == ',' || r == ';' || r == ':' ||
				r == '!' || r == '?' || r == '-' || r == '(' || r == ')' ||
				r == '[' || r == ']' || r == '{' || r == '}'
		}

		// Merge if gap is small and not punctuation
		if gap < gapThreshold && !isPunctuation {
			currentMerge = append(currentMerge, word)
		} else {
			// Finish current merge and start new one
			if len(currentMerge) > 1 {
				merged = append(merged, mergeWordGroup(currentMerge))
			} else {
				merged = append(merged, currentMerge[0])
			}
			currentMerge = []EnrichedWord{word}
		}

		// Handle last word
		if i == len(words)-1 {
			if len(currentMerge) > 1 {
				merged = append(merged, mergeWordGroup(currentMerge))
			} else {
				merged = append(merged, currentMerge[0])
			}
		}
	}

	return merged
}

// mergeWordGroup combines multiple words into a single word.
func mergeWordGroup(words []EnrichedWord) EnrichedWord {
	if len(words) == 0 {
		return EnrichedWord{}
	}
	if len(words) == 1 {
		return words[0]
	}

	// Concatenate text
	var text string
	for _, word := range words {
		text += word.Text
	}

	// Calculate merged bounding box
	box := words[0].Box
	for i := 1; i < len(words); i++ {
		box.X0 = math.Min(box.X0, words[i].Box.X0)
		box.Y0 = math.Min(box.Y0, words[i].Box.Y0)
		box.X1 = math.Max(box.X1, words[i].Box.X1)
		box.Y1 = math.Max(box.Y1, words[i].Box.Y1)
	}

	// Use first word's properties (should be similar for close words)
	return EnrichedWord{
		Text:        text,
		Box:         box,
		FontSize:    words[0].FontSize,
		FontWeight:  words[0].FontWeight,
		FontName:    words[0].FontName,
		FontFlags:   words[0].FontFlags,
		FillColor:   words[0].FillColor,
		IsBold:      words[0].IsBold,
		IsItalic:    words[0].IsItalic,
		IsMonospace: words[0].IsMonospace,
	}
}
