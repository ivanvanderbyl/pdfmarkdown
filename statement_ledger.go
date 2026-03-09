package pdfmarkdown

import (
	"math"
	"sort"
	"strings"
)

type ledgerColumn struct {
	name string
	x0   float64
	x1   float64
}

type ledgerRow struct {
	cells []string
	box   Rect
}

func preferDetectedLedgerTables(page *Page) []Table {
	if table, ok := detectStatementLedgerTable(page); ok {
		return []Table{table}
	}
	return page.Tables
}

func detectStatementLedgerTable(page *Page) (Table, bool) {
	lines := collectSortedPageLines(page)
	if len(lines) == 0 {
		return Table{}, false
	}

	headerIdx, columns := detectLedgerColumns(lines, page.Width)
	if len(columns) < 3 {
		return Table{}, false
	}

	start := headerIdx + 1
	if headerIdx == -1 {
		start = firstLedgerDataLine(lines)
	}
	if start < 0 || start >= len(lines) {
		return Table{}, false
	}

	rows, ok := buildLedgerRows(lines[start:], columns)
	if !ok || len(rows) < 1 {
		return Table{}, false
	}

	header := buildLedgerHeaderRow(columns, lines, headerIdx)
	tableRows := make([]TableRow, 0, len(rows)+1)
	tableRows = append(tableRows, header)

	tableBox := header.BBox
	for _, row := range rows {
		tableRow := ledgerRowToTableRow(row, columns)
		tableRows = append(tableRows, tableRow)
		tableBox = mergeCellBBox(tableBox, tableRow.BBox)
	}

	return Table{
		BBox:    tableBox,
		Rows:    tableRows,
		NumRows: len(tableRows),
		NumCols: len(columns),
	}, true
}

func collectSortedPageLines(page *Page) []Line {
	var lines []Line
	for _, para := range page.Paragraphs {
		lines = append(lines, para.Lines...)
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].Box.Y0 == lines[j].Box.Y0 {
			return lines[i].Box.X0 < lines[j].Box.X0
		}
		return lines[i].Box.Y0 < lines[j].Box.Y0
	})
	return lines
}

func detectLedgerColumns(lines []Line, pageWidth float64) (int, []ledgerColumn) {
	headerIdx := -1
	for i, line := range lines {
		if lineContainsLedgerHeader(line) {
			headerIdx = i
			break
		}
	}

	if headerIdx >= 0 {
		return headerIdx, columnsFromHeader(lines[headerIdx], pageWidth)
	}

	return -1, inferLedgerColumns(lines, pageWidth)
}

func columnsFromHeader(line Line, pageWidth float64) []ledgerColumn {
	type headerCell struct {
		name string
		box  Rect
	}

	var cells []headerCell
	words := line.Words
	for i := 0; i < len(words); {
		word := words[i]
		text := strings.ToLower(strings.TrimSpace(word.Text))
		switch text {
		case "date":
			cells = append(cells, headerCell{name: "Date", box: word.Box})
			i++
		case "transaction":
			box := word.Box
			name := "Transaction"
			if i+1 < len(words) && strings.EqualFold(words[i+1].Text, "details") {
				name = "Transaction details"
				box = mergeRects(box, words[i+1].Box)
				i++
			}
			cells = append(cells, headerCell{name: name, box: box})
			i++
		case "withdrawals":
			cells = append(cells, headerCell{name: "Withdrawals", box: word.Box})
			i++
		case "deposits":
			cells = append(cells, headerCell{name: "Deposits", box: word.Box})
			i++
		case "balance":
			cells = append(cells, headerCell{name: "Balance", box: word.Box})
			i++
		default:
			switch {
			case strings.Contains(text, "withdraw") || strings.Contains(text, "debit"):
				cells = append(cells, headerCell{name: "Debit/Withdrawal", box: word.Box})
			case strings.Contains(text, "deposit"):
				cells = append(cells, headerCell{name: "Deposit", box: word.Box})
			}
			i++
		}
	}

	if len(cells) < 3 {
		return nil
	}

	sort.Slice(cells, func(i, j int) bool { return cells[i].box.X0 < cells[j].box.X0 })
	columns := make([]ledgerColumn, len(cells))
	for i := range cells {
		x0 := cells[i].box.X0
		if i == 0 {
			x0 = 0
		} else {
			x0 = (cells[i-1].box.X1 + cells[i].box.X0) / 2
		}

		x1 := pageWidth
		if i < len(cells)-1 {
			x1 = (cells[i].box.X1 + cells[i+1].box.X0) / 2
		}

		columns[i] = ledgerColumn{name: cells[i].name, x0: x0, x1: x1}
	}

	return columns
}

func inferLedgerColumns(lines []Line, pageWidth float64) []ledgerColumn {
	dateBoundary := 0.0
	var amountAnchors []float64

	for _, line := range lines {
		if lineStartsWithDate(line) {
			if boundary := estimateDateBoundary(line); boundary > dateBoundary {
				dateBoundary = boundary
			}
		}
		for _, word := range line.Words {
			if looksAmountToken(word.Text) {
				if word.Box.X0 > pageWidth*0.45 {
					amountAnchors = append(amountAnchors, word.Box.X0)
				}
			}
		}
	}

	if dateBoundary == 0 || len(amountAnchors) == 0 {
		return nil
	}

	sort.Float64s(amountAnchors)
	clusters := clusterPositions(amountAnchors, 35)
	if len(clusters) == 0 {
		return nil
	}

	if len(clusters) > 3 {
		clusters = clusters[len(clusters)-3:]
	}

	columns := make([]ledgerColumn, 0, len(clusters)+2)
	columns = append(columns, ledgerColumn{name: "Date", x0: 0, x1: dateBoundary})

	descriptionEnd := clusters[0]
	columns = append(columns, ledgerColumn{name: "Transaction details", x0: dateBoundary, x1: descriptionEnd})

	amountNames := []string{"Amount", "Deposits", "Balance"}
	if len(clusters) == 3 {
		amountNames = []string{"Withdrawals", "Deposits", "Balance"}
	} else if len(clusters) == 2 {
		amountNames = []string{"Amount", "Balance"}
	}

	for i, anchor := range clusters {
		x0 := anchor
		if i > 0 {
			x0 = (clusters[i-1] + anchor) / 2
		}
		x1 := pageWidth
		if i < len(clusters)-1 {
			x1 = (anchor + clusters[i+1]) / 2
		}
		columns = append(columns, ledgerColumn{name: amountNames[i], x0: x0, x1: x1})
	}

	return columns
}

func clusterPositions(values []float64, tolerance float64) []float64 {
	if len(values) == 0 {
		return nil
	}

	clusters := []float64{values[0]}
	counts := []int{1}
	for _, v := range values[1:] {
		last := len(clusters) - 1
		if math.Abs(clusters[last]-v) <= tolerance {
			clusters[last] = (clusters[last]*float64(counts[last]) + v) / float64(counts[last]+1)
			counts[last]++
			continue
		}
		clusters = append(clusters, v)
		counts = append(counts, 1)
	}

	var filtered []float64
	for i, center := range clusters {
		if counts[i] >= 2 {
			filtered = append(filtered, center)
		}
	}
	if len(filtered) == 0 {
		return clusters
	}
	return filtered
}

func estimateDateBoundary(line Line) float64 {
	if len(line.Words) == 0 {
		return 0
	}

	last := line.Words[0].Box.X1
	limit := 2
	if len(line.Words) < limit {
		limit = len(line.Words)
	}
	for i := 0; i < limit; i++ {
		if line.Words[i].Box.X1 > last {
			last = line.Words[i].Box.X1
		}
	}
	return last + 12
}

func buildLedgerRows(lines []Line, columns []ledgerColumn) ([]ledgerRow, bool) {
	var rows []ledgerRow
	inLedger := false

	for _, line := range lines {
		if lineContainsLedgerHeader(line) {
			continue
		}
		if !inLedger {
			if !looksLikeLedgerStart(line, columns) {
				continue
			}
			inLedger = true
		}

		switch {
		case lineStartsWithDate(line), lineContainsBalanceBoundary(line):
			rows = append(rows, ledgerRowFromLine(line, columns))
			if lineStopsLedger(line) {
				return rows, true
			}
		case looksLikeLedgerContinuation(line, columns):
			if len(rows) == 0 {
				continue
			}
			appendLineToLedgerRow(&rows[len(rows)-1], line, columns)
		case isLedgerStopNoise(line) && len(rows) > 0:
			break
		default:
			if len(rows) > 0 {
				break
			}
		}
	}

	return rows, len(rows) > 0
}

func buildLedgerHeaderRow(columns []ledgerColumn, lines []Line, headerIdx int) TableRow {
	cells := make([]TableCell, len(columns))
	var rowBox Rect
	if headerIdx >= 0 {
		rowBox = lines[headerIdx].Box
	} else {
		rowBox = Rect{X0: columns[0].x0, Y0: 0, X1: columns[len(columns)-1].x1, Y1: 12}
	}
	for i, col := range columns {
		cells[i] = TableCell{
			BBox: CellBBox{X0: col.x0, Top: rowBox.Y0, X1: col.x1, Bottom: rowBox.Y1},
			Content: col.name,
		}
	}
	return TableRow{Cells: cells, BBox: rowBBoxToCellBBox(rowBox)}
}

func ledgerRowFromLine(line Line, columns []ledgerColumn) ledgerRow {
	row := ledgerRow{cells: make([]string, len(columns)), box: line.Box}
	appendWordsToLedgerRow(&row, line.Words, columns)
	return row
}

func appendLineToLedgerRow(row *ledgerRow, line Line, columns []ledgerColumn) {
	appendWordsToLedgerRow(row, line.Words, columns)
	row.box = mergeRects(row.box, line.Box)
}

func appendWordsToLedgerRow(row *ledgerRow, words []EnrichedWord, columns []ledgerColumn) {
	grouped := make([][]string, len(columns))
	for _, word := range words {
		idx := columnIndexForWord(word, columns)
		if columnLooksAmount(columns[idx]) && !looksAmountToken(word.Text) && idx > 0 {
			idx--
		}
		grouped[idx] = append(grouped[idx], word.Text)
	}
	for i := range grouped {
		if len(grouped[i]) == 0 {
			continue
		}
		text := strings.Join(grouped[i], " ")
		if row.cells[i] == "" {
			row.cells[i] = text
		} else {
			row.cells[i] += "\n" + text
		}
	}
}

func columnLooksAmount(column ledgerColumn) bool {
	name := strings.ToLower(column.name)
	return strings.Contains(name, "withdraw") ||
		strings.Contains(name, "deposit") ||
		strings.Contains(name, "balance") ||
		strings.Contains(name, "amount")
}

func columnIndexForWord(word EnrichedWord, columns []ledgerColumn) int {
	center := word.Box.CenterX()
	for i, col := range columns {
		if center >= col.x0 && center < col.x1 {
			return i
		}
	}
	return len(columns) - 1
}

func ledgerRowToTableRow(row ledgerRow, columns []ledgerColumn) TableRow {
	cells := make([]TableCell, len(columns))
	for i, col := range columns {
		cells[i] = TableCell{
			BBox:    CellBBox{X0: col.x0, Top: row.box.Y0, X1: col.x1, Bottom: row.box.Y1},
			Content: row.cells[i],
		}
	}
	return TableRow{Cells: cells, BBox: rowBBoxToCellBBox(row.box)}
}

func rowBBoxToCellBBox(box Rect) CellBBox {
	return CellBBox{X0: box.X0, Top: box.Y0, X1: box.X1, Bottom: box.Y1}
}

func mergeCellBBox(a, b CellBBox) CellBBox {
	return CellBBox{
		X0:     minFloat(a.X0, b.X0),
		Top:    minFloat(a.Top, b.Top),
		X1:     maxFloat(a.X1, b.X1),
		Bottom: maxFloat(a.Bottom, b.Bottom),
	}
}

func lineContainsLedgerHeader(line Line) bool {
	text := strings.ToLower(lineText(line))
	return strings.Contains(text, "date") &&
		strings.Contains(text, "transaction") &&
		(strings.Contains(text, "balance") || strings.Contains(text, "withdraw"))
}

func lineStartsWithDate(line Line) bool {
	if len(line.Words) < 2 {
		return false
	}
	first := strings.TrimSpace(line.Words[0].Text)
	second := strings.TrimSpace(line.Words[1].Text)
	return isDayToken(first) && isMonthToken(second)
}

func lineContainsBalanceBoundary(line Line) bool {
	text := strings.ToLower(lineText(line))
	return strings.Contains(text, "opening balance") ||
		strings.Contains(text, "closing balance") ||
		strings.Contains(text, "carried forward")
}

func lineStopsLedger(line Line) bool {
	text := strings.ToLower(lineText(line))
	return strings.Contains(text, "closing balance") || strings.Contains(text, "carried forward to next page")
}

func looksLikeLedgerStart(line Line, columns []ledgerColumn) bool {
	return lineStartsWithDate(line) || lineContainsBalanceBoundary(line) || lineLooksLikeLedgerAmounts(line, columns)
}

func looksLikeLedgerContinuation(line Line, columns []ledgerColumn) bool {
	if len(line.Words) == 0 || lineStartsWithDate(line) || lineContainsLedgerHeader(line) {
		return false
	}
	firstX := line.Words[0].Box.X0
	return firstX >= columns[1].x0-8 &&
		firstX < columns[len(columns)-1].x0 &&
		line.Box.Width() < (columns[len(columns)-1].x0-columns[1].x0)*0.9
}

func isLedgerStopNoise(line Line) bool {
	text := strings.ToLower(lineText(line))
	if strings.Contains(text, "authority") || strings.Contains(text, "borrowings") ||
		strings.Contains(text, "let's keep") || strings.Contains(text, "manage your money") {
		return true
	}
	return !lineStartsWithDate(line) && !lineContainsBalanceBoundary(line) && !looksLikeTextTableLine(line)
}

func looksLikeTextTableLine(line Line) bool {
	amounts := 0
	for _, word := range line.Words {
		if looksAmountToken(word.Text) {
			amounts++
		}
	}
	return amounts > 0
}

func lineLooksLikeLedgerAmounts(line Line, columns []ledgerColumn) bool {
	amounts := 0
	for _, word := range line.Words {
		if looksAmountToken(word.Text) {
			amounts++
		}
	}
	return amounts >= 2
}

func firstLedgerDataLine(lines []Line) int {
	for i, line := range lines {
		if lineStartsWithDate(line) || lineContainsBalanceBoundary(line) {
			return i
		}
	}
	return -1
}

func lineText(line Line) string {
	parts := make([]string, 0, len(line.Words))
	for _, word := range line.Words {
		parts = append(parts, word.Text)
	}
	return strings.Join(parts, " ")
}

func isDayToken(text string) bool {
	if len(text) == 0 || len(text) > 2 {
		return false
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isMonthToken(text string) bool {
	switch strings.ToLower(text) {
	case "jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec":
		return true
	default:
		return false
	}
}

func looksAmountToken(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	text = strings.TrimPrefix(text, "$")
	text = strings.TrimSuffix(text, "-")
	digits := 0
	for _, r := range text {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == ',' || r == '.' || r == '-':
		default:
			return false
		}
	}
	return digits >= 3 && strings.Contains(text, ".")
}
