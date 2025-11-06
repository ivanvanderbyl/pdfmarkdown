package pdfmarkdown

import (
	"math"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/pkg/errors"
)

// ExtractPage extracts all enriched text from a PDF page.
func ExtractPage(instance pdfium.Pdfium, page references.FPDF_PAGE, pageNumber int, config Config) (*Page, error) {
	// Get page dimensions
	pageSize, err := instance.FPDF_GetPageWidthF(&requests.FPDF_GetPageWidthF{
		Page: requests.Page{
			ByReference: &page,
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get page width")
	}

	pageHeight, err := instance.FPDF_GetPageHeightF(&requests.FPDF_GetPageHeightF{
		Page: requests.Page{
			ByReference: &page,
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get page size")
	}

	// Get MediaBox to handle non-zero origins
	// For now, assume origin at (0,0) - MediaBox support can be added when needed
	// Most PDFs have MediaBox starting at origin
	originX := 0.0
	originY := 0.0

	// Load text page
	textPage, err := instance.FPDFText_LoadPage(&requests.FPDFText_LoadPage{
		Page: requests.Page{
			ByReference: &page,
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to load text page")
	}
	defer instance.FPDFText_ClosePage(&requests.FPDFText_ClosePage{
		TextPage: textPage.TextPage,
	})

	// Count characters
	charCount, err := instance.FPDFText_CountChars(&requests.FPDFText_CountChars{
		TextPage: textPage.TextPage,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to count characters")
	}

	if charCount.Count == 0 {
		return &Page{
			Number:     pageNumber,
			Width:      float64(pageSize.PageWidth),
			Height:     float64(pageHeight.PageHeight),
			Paragraphs: []Paragraph{},
		}, nil
	}

	// Extract all characters with metadata
	chars, err := extractEnrichedChars(instance, textPage.TextPage, charCount.Count, float64(pageHeight.PageHeight))
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract characters")
	}

	// Normalize coordinates by MediaBox origin
	for i := range chars {
		chars[i].Box.X0 -= originX
		chars[i].Box.X1 -= originX
		chars[i].Box.Y0 -= originY
		chars[i].Box.Y1 -= originY
	}

	// Group characters into words
	words := groupCharsIntoWords(chars)

	// Expand ligatures
	words = expandLigatures(words)

	// Deduplicate CJK characters
	words = deduplicateCJKChars(words)

	// Build document structure
	// Note: Word merging based on proximity happens in buildParagraphs after line grouping
	paragraphs := buildParagraphs(words, float64(pageSize.PageWidth), config)

	// Extract explicit line objects from the PDF
	lines, err := extractLinesFromPage(instance, page, float64(pageSize.PageWidth), float64(pageHeight.PageHeight))
	if err != nil {
		// Non-fatal: continue without lines
		lines = []Edge{}
	}

	// Detect columns
	columns := detectColumns(words, float64(pageSize.PageWidth))

	// Create page with paragraphs
	resultPage := &Page{
		Number:     pageNumber,
		Width:      float64(pageSize.PageWidth),
		Height:     float64(pageHeight.PageHeight),
		Paragraphs: paragraphs,
		Lines:      lines,
		Columns:    columns,
	}

	// Detect tables if enabled
	if config.DetectTables {
		tables := DetectTables(resultPage, config.TableSettings)
		resultPage.Tables = tables
	}

	return resultPage, nil
}

// extractEnrichedChars extracts all characters with their metadata.
func extractEnrichedChars(instance pdfium.Pdfium, textPage references.FPDF_TEXTPAGE, count int, pageHeight float64) ([]EnrichedChar, error) {
	chars := make([]EnrichedChar, 0, count)

	for i := range count {
		// Get Unicode character
		unicodeRes, err := instance.FPDFText_GetUnicode(&requests.FPDFText_GetUnicode{
			TextPage: textPage,
			Index:    i,
		})
		if err != nil || unicodeRes.Unicode == 0 {
			continue
		}

		// Get bounding box
		charBox, err := instance.FPDFText_GetCharBox(&requests.FPDFText_GetCharBox{
			TextPage: textPage,
			Index:    i,
		})
		if err != nil {
			continue
		}

		// Convert PDF coordinates (origin bottom-left) to standard (origin top-left)
		box := Rect{
			X0: charBox.Left,
			Y0: pageHeight - charBox.Top,
			X1: charBox.Right,
			Y1: pageHeight - charBox.Bottom,
		}

		// Get font size
		fontSize, err := instance.FPDFText_GetFontSize(&requests.FPDFText_GetFontSize{
			TextPage: textPage,
			Index:    i,
		})
		fontSizeVal := 12.0 // Default
		if err == nil {
			fontSizeVal = fontSize.FontSize
		}

		// Get font weight
		fontWeight, err := instance.FPDFText_GetFontWeight(&requests.FPDFText_GetFontWeight{
			TextPage: textPage,
			Index:    i,
		})
		fontWeightVal := 400 // Default normal weight
		if err == nil {
			fontWeightVal = fontWeight.FontWeight
		}

		// Get font info
		fontInfo, err := instance.FPDFText_GetFontInfo(&requests.FPDFText_GetFontInfo{
			TextPage: textPage,
			Index:    i,
		})
		fontNameVal := ""
		fontFlagsVal := 0
		if err == nil {
			fontNameVal = fontInfo.FontName
			fontFlagsVal = fontInfo.Flags
		}

		// Get fill color
		fillColor, err := instance.FPDFText_GetFillColor(&requests.FPDFText_GetFillColor{
			TextPage: textPage,
			Index:    i,
		})
		fillColorVal := RGBA{R: 0, G: 0, B: 0, A: 255} // Default black
		if err == nil {
			fillColorVal = RGBA{
				R: fillColor.R,
				G: fillColor.G,
				B: fillColor.B,
				A: fillColor.A,
			}
		}

		// Get angle
		angle, err := instance.FPDFText_GetCharAngle(&requests.FPDFText_GetCharAngle{
			TextPage: textPage,
			Index:    i,
		})
		angleVal := float32(0)
		if err == nil {
			angleVal = angle.CharAngle
		}

		// Check if hyphen
		isHyphen, err := instance.FPDFText_IsHyphen(&requests.FPDFText_IsHyphen{
			TextPage: textPage,
			Index:    i,
		})
		isHyphenVal := false
		if err == nil {
			isHyphenVal = isHyphen.IsHyphen
		}

		chars = append(chars, EnrichedChar{
			Text:       rune(unicodeRes.Unicode),
			Box:        box,
			FontSize:   fontSizeVal,
			FontWeight: fontWeightVal,
			FontName:   fontNameVal,
			FontFlags:  fontFlagsVal,
			FillColor:  fillColorVal,
			Angle:      angleVal,
			IsHyphen:   isHyphenVal,
		})
	}

	return chars, nil
}

// groupCharsIntoWords groups characters into words based on spacing.
func groupCharsIntoWords(chars []EnrichedChar) []EnrichedWord {
	if len(chars) == 0 {
		return nil
	}

	var words []EnrichedWord
	var currentWord []EnrichedChar
	var wordBox Rect
	wordStarted := false

	for i, char := range chars {
		isWhitespace := char.Text == ' ' || char.Text == '\t' || char.Text == '\n' || char.Text == '\r'

		if !isWhitespace {
			if !wordStarted {
				wordBox = char.Box
				wordStarted = true
			} else {
				// Expand bounding box
				wordBox.X0 = math.Min(wordBox.X0, char.Box.X0)
				wordBox.Y0 = math.Min(wordBox.Y0, char.Box.Y0)
				wordBox.X1 = math.Max(wordBox.X1, char.Box.X1)
				wordBox.Y1 = math.Max(wordBox.Y1, char.Box.Y1)
			}
			currentWord = append(currentWord, char)
		}

		// End word on whitespace or end of text
		if (isWhitespace || i == len(chars)-1) && len(currentWord) > 0 {
			words = append(words, aggregateWord(currentWord, wordBox))
			currentWord = nil
			wordStarted = false
		}
	}

	return words
}

// aggregateWord creates an EnrichedWord from a slice of characters.
func aggregateWord(chars []EnrichedChar, box Rect) EnrichedWord {
	if len(chars) == 0 {
		return EnrichedWord{}
	}

	// Build text
	var text string
	for _, char := range chars {
		text += string(char.Text)
	}

	// Calculate average font size
	var totalFontSize float64
	for _, char := range chars {
		totalFontSize += char.FontSize
	}
	avgFontSize := totalFontSize / float64(len(chars))

	// Find dominant font weight (most common)
	weightCounts := make(map[int]int)
	for _, char := range chars {
		weightCounts[char.FontWeight]++
	}
	var dominantWeight int
	var maxCount int
	for weight, count := range weightCounts {
		if count > maxCount {
			dominantWeight = weight
			maxCount = count
		}
	}

	// Find dominant font name
	fontCounts := make(map[string]int)
	for _, char := range chars {
		fontCounts[char.FontName]++
	}
	var dominantFont string
	maxCount = 0
	for font, count := range fontCounts {
		if count > maxCount {
			dominantFont = font
			maxCount = count
		}
	}

	// Get first char's font flags (usually consistent within a word)
	fontFlags := chars[0].FontFlags

	// Determine style flags
	isBold := dominantWeight >= 700
	isItalic := (fontFlags & 0x40) != 0    // Italic flag from PDF spec
	isMonospace := (fontFlags & 0x01) != 0 // FixedPitch flag

	// Calculate average rotation angle
	var totalAngle float64
	for _, char := range chars {
		totalAngle += float64(char.Angle)
	}
	avgAngle := totalAngle / float64(len(chars))

	word := EnrichedWord{
		Text:        text,
		Box:         box,
		FontSize:    avgFontSize,
		FontWeight:  dominantWeight,
		FontName:    dominantFont,
		FontFlags:   fontFlags,
		FillColor:   chars[0].FillColor,
		IsBold:      isBold,
		IsItalic:    isItalic,
		IsMonospace: isMonospace,
		Rotation:    float64(avgAngle) * 180 / 3.14159, // Convert radians to degrees
	}

	// Calculate baseline and x-height
	word.Baseline = calculateBaseline(word)
	word.XHeight = calculateXHeight(word)

	return word
}

// ligatureMap maps ligature unicode codepoints to their expanded forms
var ligatureMap = map[rune]string{
	0xFB00: "ff",
	0xFB01: "fi",
	0xFB02: "fl",
	0xFB03: "ffi",
	0xFB04: "ffl",
	0xFB05: "ft",
	0xFB06: "st",
}

// expandLigatures expands ligature characters into their component letters
func expandLigatures(words []EnrichedWord) []EnrichedWord {
	for i := range words {
		word := &words[i]
		runes := []rune(word.Text)
		hasLigature := false

		// Check if word contains any ligatures
		for _, r := range runes {
			if _, isLigature := ligatureMap[r]; isLigature {
				hasLigature = true
				break
			}
		}

		if !hasLigature {
			continue
		}

		// Expand ligatures
		var expanded []rune
		for _, r := range runes {
			if expansion, isLigature := ligatureMap[r]; isLigature {
				expanded = append(expanded, []rune(expansion)...)
			} else {
				expanded = append(expanded, r)
			}
		}

		word.Text = string(expanded)
	}
	return words
}

// isCJK checks if a rune is in a CJK unicode block
func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Unified Ideographs Extension A
		(r >= 0x20000 && r <= 0x2A6DF) || // CJK Unified Ideographs Extension B
		(r >= 0x2A700 && r <= 0x2B73F) || // CJK Unified Ideographs Extension C
		(r >= 0x2B740 && r <= 0x2B81F) || // CJK Unified Ideographs Extension D
		(r >= 0x2B820 && r <= 0x2CEAF) || // CJK Unified Ideographs Extension E
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility Ideographs
		(r >= 0x2F800 && r <= 0x2FA1F) // CJK Compatibility Ideographs Supplement
}

// containsCJK checks if a slice of runes contains any CJK characters
func containsCJK(runes []rune) bool {
	for _, r := range runes {
		if isCJK(r) {
			return true
		}
	}
	return false
}

// deduplicateCJKChars removes duplicate consecutive CJK characters that appear
// at nearly identical positions (common rendering artifact in some PDFs)
func deduplicateCJKChars(words []EnrichedWord) []EnrichedWord {
	for i := range words {
		word := &words[i]
		runes := []rune(word.Text)

		// Only process words containing CJK characters
		if !containsCJK(runes) {
			continue
		}

		if len(runes) <= 1 {
			continue
		}

		// Build deduplicated text by checking consecutive identical characters
		deduplicated := []rune{runes[0]}

		for j := 1; j < len(runes); j++ {
			// Check if current character is identical to previous AND is CJK
			if runes[j] == runes[j-1] && isCJK(runes[j]) {
				// Calculate approximate horizontal spacing
				// Since we've already grouped into words, we use the word width
				// divided by character count as an approximation
				avgCharWidth := word.Box.Width() / float64(len(runes))

				// If this looks like a duplicate (same char, CJK, typical spacing suggests overlap)
				// Skip it. This heuristic catches cases like "微微软软" -> "微软"
				if avgCharWidth < word.FontSize*0.7 {
					continue
				}
			}

			deduplicated = append(deduplicated, runes[j])
		}

		word.Text = string(deduplicated)
	}
	return words
}
