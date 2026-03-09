package pdfmarkdown

import (
	"bytes"
	"math"
	"sort"
	"strings"

	"github.com/ivanvanderbyl/markdown"
)

// ToMarkdown converts a document to markdown format.
func (d *Document) ToMarkdown(config Config) string {
	// Normalize heading levels across the entire document
	normalizeDocumentHeadings(d)

	var buf bytes.Buffer
	md := markdown.NewMarkdown(&buf)

	for i, page := range d.Pages {
		if i > 0 && config.IncludePageBreaks {
			md.HorizontalRule().LF()
		}

		renderPageContent(md, page, config)
	}

	if err := md.Build(); err != nil {
		// If there's an error building the markdown, fall back to empty string
		return ""
	}

	return buf.String()
}

// normalizeDocumentHeadings adjusts heading levels across all pages to be consistent
// This ensures H1 is the largest heading across the entire document, not just within a page
func normalizeDocumentHeadings(doc *Document) {
	// Collect all heading font sizes across all pages
	type HeadingInfo struct {
		fontSize float64
		pageIdx  int
		paraIdx  int
	}

	var headings []HeadingInfo
	fontSizeSet := make(map[float64]bool)

	for pi, page := range doc.Pages {
		for pri, para := range page.Paragraphs {
			if para.IsHeading && len(para.Lines) > 0 && len(para.Lines[0].Words) > 0 {
				// Get max font size of the heading
				var maxSize float64
				for _, word := range para.Lines[0].Words {
					if word.FontSize > maxSize {
						maxSize = word.FontSize
					}
				}

				headings = append(headings, HeadingInfo{
					fontSize: maxSize,
					pageIdx:  pi,
					paraIdx:  pri,
				})
				fontSizeSet[maxSize] = true
			}
		}
	}

	if len(fontSizeSet) == 0 {
		return
	}

	// Create sorted list of unique font sizes (descending)
	var uniqueSizes []float64
	for size := range fontSizeSet {
		uniqueSizes = append(uniqueSizes, size)
	}
	sort.Float64s(uniqueSizes)
	// Reverse to descending
	for i := 0; i < len(uniqueSizes)/2; i++ {
		j := len(uniqueSizes) - 1 - i
		uniqueSizes[i], uniqueSizes[j] = uniqueSizes[j], uniqueSizes[i]
	}

	// Map font sizes to heading levels (largest = H1, etc.)
	sizeToLevel := make(map[float64]int)
	for i, size := range uniqueSizes {
		if i < 6 {
			sizeToLevel[size] = i + 1
		} else {
			sizeToLevel[size] = 6 // Max H6
		}
	}

	// Apply normalized levels to all headings
	for _, h := range headings {
		if level, ok := sizeToLevel[h.fontSize]; ok {
			doc.Pages[h.pageIdx].Paragraphs[h.paraIdx].HeadingLevel = level
		}
	}
}

// convertParagraphToMarkdown converts a single paragraph to markdown using the builder.
func convertParagraphToMarkdown(md *markdown.Markdown, para Paragraph) {
	if len(para.Lines) == 0 {
		return
	}

	// Handle headings
	if para.IsHeading {
		// For multi-line paragraphs marked as headings, only the first line is the heading
		// The rest should be rendered as regular text
		if len(para.Lines) > 1 {
			// Render first line as heading
			firstLineText := ""
			for j, word := range para.Lines[0].Words {
				if j > 0 {
					firstLineText += " "
				}
				firstLineText += word.Text
			}
			firstLineText = strings.TrimRight(firstLineText, " \t")

			switch para.HeadingLevel {
			case 1:
				md.H1(firstLineText)
			case 2:
				md.H2(firstLineText)
			case 3:
				md.H3(firstLineText)
			case 4:
				md.H4(firstLineText)
			case 5:
				md.H5(firstLineText)
			case 6:
				md.H6(firstLineText)
			default:
				md.H1(firstLineText)
			}

			// Render remaining lines as regular paragraph
			// Create a temporary non-heading paragraph for the rest
			restPara := Paragraph{
				Lines:     para.Lines[1:],
				Box:       para.Box,
				IsHeading: false,
			}
			md.LF()
			convertParagraphToMarkdown(md, restPara)
		} else {
			// Single-line heading - render normally
			text := strings.TrimRight(para.Text(), " \t")
			switch para.HeadingLevel {
			case 1:
				md.H1(text)
			case 2:
				md.H2(text)
			case 3:
				md.H3(text)
			case 4:
				md.H4(text)
			case 5:
				md.H5(text)
			case 6:
				md.H6(text)
			default:
				md.H1(text)
			}
		}
		return
	}

	// Handle code blocks
	if para.IsCode {
		text := para.Text()
		// Trim trailing whitespace from each line
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			lines[i] = strings.TrimRight(line, " \t")
		}
		text = strings.Join(lines, "\n")
		md.CodeBlocks(markdown.SyntaxHighlightNone, text)
		return
	}

	// Handle lists
	if para.IsList {
		text := strings.TrimRight(para.Text(), " \t")
		// Check if it's a numbered list
		if len(text) > 0 && (text[0] >= '0' && text[0] <= '9') {
			// Extract the list item text (after the number and period)
			parts := strings.SplitN(text, ".", 2)
			if len(parts) == 2 {
				md.OrderedList(strings.TrimSpace(parts[1]))
			} else {
				md.OrderedList(text)
			}
		} else {
			// Bullet list - remove any existing bullet prefix
			text = strings.TrimPrefix(text, "* ")
			text = strings.TrimPrefix(text, "- ")
			text = strings.TrimPrefix(text, "+ ")
			md.BulletList(text)
		}
		return
	}

	// Handle regular paragraphs with inline formatting
	// Special handling: split on numbered items for better readability
	var currentSection strings.Builder
	sections := []string{}

	for _, line := range para.Lines {
		// Check if this line starts with a numbered item (2., 3., 4., etc.)
		startsWithNumber := false
		if len(line.Words) > 0 {
			firstWord := line.Words[0].Text
			if len(firstWord) >= 2 && firstWord[0] >= '2' && firstWord[0] <= '9' && firstWord[1] == '.' {
				startsWithNumber = true
			}
		}

		// If we hit a new numbered section (and we have content), save current section
		if startsWithNumber && currentSection.Len() > 0 {
			sections = append(sections, strings.TrimRight(currentSection.String(), " \t"))
			currentSection.Reset()
		}

		// Add line break before this line (unless it's the first line or start of new section)
		if currentSection.Len() > 0 {
			currentSection.WriteString("  \n")
		}

		// Build the line content using formatting runs to avoid per-word markers
		currentSection.WriteString(buildFormattedLine(line.Words))
	}

	// Add final section
	if currentSection.Len() > 0 {
		sections = append(sections, strings.TrimRight(currentSection.String(), " \t"))
	}

	// Output sections with visual separation
	if len(sections) == 1 {
		// Single section - output normally
		md.PlainText(sections[0])
	} else if len(sections) > 1 {
		// Multiple sections - add blank lines between numbered items
		for si, section := range sections {
			if section != "" {
				// Output the section
				md.PlainText(section)

				// Add visual separator after each section except the last
				if si < len(sections)-1 {
					md.LF() // End current section, creating blank line before next section
				}
			}
		}
	}
}

// wordStyle returns a comparable key representing the inline formatting of a word.
type inlineStyle struct {
	bold, italic, mono bool
}

func wordStyle(w EnrichedWord) inlineStyle {
	return inlineStyle{bold: w.IsBold, italic: w.IsItalic, mono: w.IsMonospace}
}

// buildFormattedLine groups consecutive words with the same formatting style
// into runs so that e.g. bold spans produce a single **bold run** instead of
// **word1** **word2** **word3**.
func buildFormattedLine(words []EnrichedWord) string {
	if len(words) == 0 {
		return ""
	}

	var out strings.Builder

	runStart := 0
	runStyle := wordStyle(words[0])

	flushRun := func(end int) {
		var runText strings.Builder
		for i := runStart; i < end; i++ {
			if i > runStart {
				runText.WriteString(" ")
			}
			runText.WriteString(words[i].Text)
		}

		text := runText.String()
		if runStyle.bold && runStyle.italic {
			text = markdown.BoldItalic(text)
		} else if runStyle.bold {
			text = markdown.Bold(text)
		} else if runStyle.italic {
			text = markdown.Italic(text)
		} else if runStyle.mono {
			text = markdown.Code(text)
		}

		if out.Len() > 0 {
			out.WriteString(" ")
		}
		out.WriteString(text)
	}

	for i := 1; i < len(words); i++ {
		s := wordStyle(words[i])
		if s != runStyle {
			flushRun(i)
			runStart = i
			runStyle = s
		}
	}
	flushRun(len(words))

	return out.String()
}

// applyInlineFormatting applies markdown formatting to a word based on its style.
func applyInlineFormatting(word EnrichedWord) string {
	text := word.Text

	if word.IsBold && word.IsItalic {
		return markdown.BoldItalic(text)
	}

	if word.IsBold {
		return markdown.Bold(text)
	}

	if word.IsItalic {
		return markdown.Italic(text)
	}

	if word.IsMonospace {
		return markdown.Code(text)
	}

	return text
}

// convertTableToMarkdown converts a table to markdown format using the builder.
func convertTableToMarkdown(md *markdown.Markdown, table Table) {
	if len(table.Rows) == 0 {
		return
	}

	// Convert table rows to string slices for the markdown builder
	var header []string
	var rows [][]string

	for rowIdx, row := range table.Rows {
		cells := make([]string, table.NumCols)
		for colIdx := 0; colIdx < table.NumCols; colIdx++ {
			if colIdx < len(row.Cells) {
				// Replace newlines with spaces in cell content
				cells[colIdx] = strings.ReplaceAll(row.Cells[colIdx].Content, "\n", " ")
			} else {
				cells[colIdx] = ""
			}
		}

		if rowIdx == 0 {
			// First row is the header
			header = cells
		} else {
			rows = append(rows, cells)
		}
	}

	// If we only have a header and no data rows, still create a valid table
	if len(rows) == 0 && len(header) > 0 {
		rows = [][]string{make([]string, len(header))}
	}

	md.Table(markdown.TableSet{
		Header: header,
		Rows:   rows,
	})
}

// PageToMarkdown converts a single page to markdown.
func (p *Page) ToMarkdown() string {
	var buf bytes.Buffer
	md := markdown.NewMarkdown(&buf)

	renderPageContent(md, *p, DefaultConfig())

	if err := md.Build(); err != nil {
		// If there's an error building the markdown, fall back to empty string
		return ""
	}

	return buf.String()
}

type pageRenderItem struct {
	top   float64
	kind  string
	para  Paragraph
	table Table
}

func renderPageContent(md *markdown.Markdown, page Page, config Config) {
	items := planPageRenderItems(page, config)

	sort.Slice(items, func(i, j int) bool {
		if items[i].top == items[j].top {
			return items[i].kind < items[j].kind
		}
		return items[i].top < items[j].top
	})

	for _, item := range items {
		if item.kind == "table" {
			convertTableToMarkdown(md, item.table)
		} else if len(item.para.Lines) > 0 {
			convertParagraphToMarkdown(md, item.para)
		}
		md.LF()
	}
}

type tableRenderPlan struct {
	table Table
	label *Paragraph
}

func planPageRenderItems(page Page, config Config) []pageRenderItem {
	var items []pageRenderItem
	plans, consumed := planRenderableTables(page, config)

	for idx, para := range page.Paragraphs {
		if consumed[idx] {
			continue
		}
		if config.DetectTables && paragraphOverlapsAnyTable(para, page.Tables) {
			continue
		}
		items = append(items, pageRenderItem{
			top:  para.Box.Y0,
			kind: "paragraph",
			para: para,
		})
	}

	if !config.DetectTables {
		return items
	}

	for _, plan := range plans {
		if plan.label != nil {
			items = append(items, pageRenderItem{
				top:  plan.label.Box.Y0,
				kind: "paragraph",
				para: *plan.label,
			})
		}
		items = append(items, pageRenderItem{
			top:   plan.table.BBox.Top,
			kind:  "table",
			table: plan.table,
		})
	}

	return items
}

func planRenderableTables(page Page, config Config) ([]tableRenderPlan, map[int]bool) {
	consumed := make(map[int]bool)
	if !config.DetectTables || len(page.Tables) == 0 {
		return nil, consumed
	}

	tables := sortedTables(page.Tables)
	if len(tables) == 0 {
		return nil, consumed
	}

	if !isHeaderOnlyTable(tables[0]) || !hasMatchingSchemaFollower(tables[0], tables[1:]) {
		return defaultTablePlans(tables), consumed
	}

	anchor := tables[0]
	plans := make([]tableRenderPlan, 0, len(tables)-1)
	var current *tableRenderPlan
	prevBottom := anchor.BBox.Bottom
	usedAnchor := false

	for i := 1; i < len(tables); i++ {
		table := tables[i]
		if !sharesTableSchema(anchor, table) || isHeaderOnlyTable(table) {
			if current != nil {
				plans = append(plans, *current)
				current = nil
			}
			plans = append(plans, tableRenderPlan{table: table})
			prevBottom = table.BBox.Bottom
			continue
		}

		usedAnchor = true
		label, labelIdx, hasLabel := findSectionLabelForTable(page.Paragraphs, table)
		if hasLabel {
			consumed[labelIdx] = true
		}

		boundary := hasLabel || hasInterveningBoundaryParagraph(page.Paragraphs, page.Tables, prevBottom, table.BBox.Top, consumed)
		if current == nil || boundary {
			if current != nil {
				plans = append(plans, *current)
			}
			sectionTable := copyHeaderToTable(anchor, table)
			current = &tableRenderPlan{table: sectionTable}
			if hasLabel {
				labelCopy := label
				current.label = &labelCopy
			}
		} else {
			current.table = mergeTables(current.table, table)
		}

		prevBottom = table.BBox.Bottom
	}

	if current != nil {
		plans = append(plans, *current)
	}

	if !usedAnchor {
		return append([]tableRenderPlan{{table: anchor}}, plans...), consumed
	}

	return plans, consumed
}

func defaultTablePlans(tables []Table) []tableRenderPlan {
	plans := make([]tableRenderPlan, 0, len(tables))
	for _, table := range tables {
		plans = append(plans, tableRenderPlan{table: table})
	}
	return plans
}

func sortedTables(tables []Table) []Table {
	sorted := make([]Table, len(tables))
	copy(sorted, tables)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].BBox.Top < sorted[j].BBox.Top
	})
	return sorted
}

func hasMatchingSchemaFollower(anchor Table, candidates []Table) bool {
	for _, candidate := range candidates {
		if sharesTableSchema(anchor, candidate) && !isHeaderOnlyTable(candidate) {
			return true
		}
	}
	return false
}

func sharesTableSchema(anchor, candidate Table) bool {
	return anchor.NumCols > 0 && candidate.NumCols == anchor.NumCols
}

func copyHeaderToTable(anchor, table Table) Table {
	if len(anchor.Rows) == 0 {
		return table
	}

	headerRows := append([]TableRow{}, anchor.Rows...)
	rows := make([]TableRow, 0, len(headerRows)+len(table.Rows))
	rows = append(rows, headerRows...)
	rows = append(rows, table.Rows...)

	box := table.BBox
	if anchor.BBox.X0 < box.X0 {
		box.X0 = anchor.BBox.X0
	}
	if anchor.BBox.X1 > box.X1 {
		box.X1 = anchor.BBox.X1
	}

	cells := append([]CellBBox{}, anchor.Cells...)
	cells = append(cells, table.Cells...)

	return Table{
		BBox:    box,
		Rows:    rows,
		Cells:   cells,
		NumRows: len(rows),
		NumCols: table.NumCols,
	}
}

func hasInterveningBoundaryParagraph(paragraphs []Paragraph, tables []Table, fromY, toY float64, consumed map[int]bool) bool {
	for idx, para := range paragraphs {
		if consumed[idx] {
			continue
		}
		if para.Box.Y1 <= fromY || para.Box.Y0 >= toY {
			continue
		}
		if strings.TrimSpace(para.Text()) == "" {
			continue
		}
		if paragraphOverlapsAnyTable(para, tables) {
			continue
		}
		return true
	}

	return false
}

func findSectionLabelForTable(paragraphs []Paragraph, table Table) (Paragraph, int, bool) {
	bestIdx := -1
	bestScore := math.MaxFloat64
	var bestLabel Paragraph

	for idx, para := range paragraphs {
		label, ok := extractSectionLabelParagraph(para)
		if !ok {
			continue
		}
		if para.Box.Y0 > table.BBox.Bottom || para.Box.Y1 < table.BBox.Top-18 {
			continue
		}

		score := math.Abs(label.Box.Y0 - table.BBox.Top)
		if score < bestScore {
			bestScore = score
			bestIdx = idx
			bestLabel = label
		}
	}

	if bestIdx == -1 {
		return Paragraph{}, -1, false
	}

	return bestLabel, bestIdx, true
}

func extractSectionLabelParagraph(para Paragraph) (Paragraph, bool) {
	if len(para.Lines) < 2 || !lineLooksLikeSectionLabel(para.Lines[0]) || !paragraphLooksLikeSectionBody(para) {
		return Paragraph{}, false
	}

	return Paragraph{
		Lines:     []Line{para.Lines[0]},
		Box:       para.Lines[0].Box,
		Alignment: para.Alignment,
	}, true
}

func lineLooksLikeSectionLabel(line Line) bool {
	if len(line.Words) == 0 || len(line.Words) > 5 {
		return false
	}

	for _, word := range line.Words {
		text := strings.TrimSpace(word.Text)
		if text == "" || looksDataLikeCell(text) {
			return false
		}
	}

	return true
}

func paragraphLooksLikeSectionBody(para Paragraph) bool {
	if len(para.Lines) < 2 {
		return false
	}

	labelWidth := para.Lines[0].Box.Width()
	for _, line := range para.Lines[1:] {
		if line.Box.Width() > labelWidth*1.5 {
			return true
		}
		for _, word := range line.Words {
			if looksDataLikeCell(word.Text) {
				return true
			}
		}
	}

	return false
}

func isHeaderOnlyTable(table Table) bool {
	if table.NumCols < 2 || len(table.Rows) != 1 {
		return false
	}

	nonEmpty := 0
	for _, cell := range table.Rows[0].Cells {
		content := strings.TrimSpace(cell.Content)
		if content == "" {
			continue
		}
		nonEmpty++
		if looksDataLikeCell(content) {
			return false
		}
	}

	return nonEmpty >= max(2, table.NumCols-1)
}
func mergeTables(anchor, candidate Table) Table {
	mergedRows := make([]TableRow, 0, len(anchor.Rows)+len(candidate.Rows))
	mergedRows = append(mergedRows, anchor.Rows...)
	mergedRows = append(mergedRows, candidate.Rows...)

	mergedBox := anchor.BBox
	if candidate.BBox.X0 < mergedBox.X0 {
		mergedBox.X0 = candidate.BBox.X0
	}
	if candidate.BBox.Top < mergedBox.Top {
		mergedBox.Top = candidate.BBox.Top
	}
	if candidate.BBox.X1 > mergedBox.X1 {
		mergedBox.X1 = candidate.BBox.X1
	}
	if candidate.BBox.Bottom > mergedBox.Bottom {
		mergedBox.Bottom = candidate.BBox.Bottom
	}

	mergedCells := append([]CellBBox{}, anchor.Cells...)
	mergedCells = append(mergedCells, candidate.Cells...)

	return Table{
		BBox:    mergedBox,
		Rows:    mergedRows,
		Cells:   mergedCells,
		NumRows: len(mergedRows),
		NumCols: anchor.NumCols,
	}
}

func looksDataLikeCell(content string) bool {
	for _, r := range content {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return strings.TrimSpace(content) == "-"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func paragraphOverlapsAnyTable(para Paragraph, tables []Table) bool {
	for _, table := range tables {
		if paragraphTableOverlapRatio(para.Box, table.BBox) >= 0.4 {
			return true
		}
	}
	return false
}

func paragraphTableOverlapRatio(box Rect, table CellBBox) float64 {
	x0 := maxFloat(box.X0, table.X0)
	y0 := maxFloat(box.Y0, table.Top)
	x1 := minFloat(box.X1, table.X1)
	y1 := minFloat(box.Y1, table.Bottom)
	if x1 <= x0 || y1 <= y0 {
		return 0
	}

	intersection := (x1 - x0) * (y1 - y0)
	area := box.Width() * box.Height()
	if area <= 0 {
		return 0
	}

	return intersection / area
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
