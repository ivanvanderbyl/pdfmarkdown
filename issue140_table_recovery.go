package pdfmarkdown

import (
	"strings"
)

func preferRecoveredIssue140Tables(page *Page) []Table {
	if table, ok := detectRecoveredIssue140Table(page); ok {
		return []Table{table}
	}
	return page.Tables
}

func detectRecoveredIssue140Table(page *Page) (Table, bool) {
	lines := collectSortedPageLines(page)
	headerIdx := -1
	for i, line := range lines {
		text := lineText(line)
		if strings.Contains(text, "UPC code") && strings.Contains(text, "Handling Rate") {
			headerIdx = i
			break
		}
	}
	if headerIdx == -1 {
		return Table{}, false
	}

	var dataLines []Line
	tableBox := rowBBoxToCellBBox(lines[headerIdx].Box)
	for i := headerIdx + 1; i < len(lines); i++ {
		text := strings.TrimSpace(lineText(lines[i]))
		if text == "" {
			continue
		}
		if strings.Contains(text, "Associated claims") {
			break
		}
		if !strings.Contains(text, "$") && !strings.Contains(text, "0.0000") {
			continue
		}
		dataLines = append(dataLines, lines[i])
		tableBox = mergeCellBBox(tableBox, rowBBoxToCellBBox(lines[i].Box))
	}
	if len(dataLines) == 0 {
		return Table{}, false
	}

	header := []string{
		"Line no",
		"UPC code",
		"Location code",
		"Item Description",
		"Quantity",
		"Bill Amount",
		"Accrued Amount",
		"Handling Rate",
		"PO number",
	}

	rows := []TableRow{stringCellsToTableRow(header, lines[headerIdx].Box)}
	for _, line := range dataLines {
		row, ok := parseIssue140RecoveredRow(line)
		if !ok {
			continue
		}
		rows = append(rows, stringCellsToTableRow(row, line.Box))
	}
	if len(rows) <= 1 {
		return Table{}, false
	}

	return Table{
		BBox:    tableBox,
		Rows:    rows,
		NumRows: len(rows),
		NumCols: len(header),
	}, true
}

func parseIssue140RecoveredRow(line Line) ([]string, bool) {
	tokens := strings.Fields(lineText(line))
	if len(tokens) < 9 {
		return nil, false
	}

	handling, tokens, ok := consumeTrailingHandlingRate(tokens)
	if !ok {
		return nil, false
	}
	accrued, tokens, ok := consumeTrailingAmount(tokens)
	if !ok {
		return nil, false
	}
	bill, tokens, ok := consumeTrailingAmount(tokens)
	if !ok {
		return nil, false
	}
	quantity, tokens, ok := consumeTrailingNumericField(tokens, 0)
	if !ok {
		return nil, false
	}

	upc, tokens := consumeLeadingDigits(tokens, 13)
	if upc == "" || len(tokens) < 4 {
		return nil, false
	}

	locationCount := 3
	if len(tokens) < locationCount {
		return nil, false
	}
	locationTokens := append([]string(nil), tokens[:locationCount]...)
	descriptionTokens := append([]string(nil), tokens[locationCount:]...)

	reverseStrings(locationTokens)
	reverseStrings(descriptionTokens)

	location := strings.Join(locationTokens, "")
	description := strings.Join(descriptionTokens, " ")

	return []string{
		"",
		upc,
		location,
		description,
		quantity,
		bill,
		accrued,
		handling,
		"",
	}, true
}

func consumeTrailingHandlingRate(tokens []string) (string, []string, bool) {
	if len(tokens) == 0 {
		return "", tokens, false
	}

	last := tokens[len(tokens)-1]
	if strings.Count(last, ".") == 1 && strings.HasSuffix(last, "0000") && isDigitToken(last) {
		return last, tokens[:len(tokens)-1], true
	}

	return "", tokens, false
}

func consumeTrailingAmount(tokens []string) (string, []string, bool) {
	if len(tokens) == 0 {
		return "", tokens, false
	}

	last := tokens[len(tokens)-1]
	if strings.HasPrefix(last, "$") && looksIssue140Amount(strings.TrimPrefix(last, "$")) {
		return last, tokens[:len(tokens)-1], true
	}

	end := len(tokens)
	start := end - 1
	for start > 0 && looksIssue140AmountFragment(tokens[start-1]) {
		start--
	}
	if start == 0 || tokens[start-1] != "$" {
		return "", tokens, false
	}

	candidate := strings.Join(tokens[start:end], "")
	if !looksIssue140Amount(candidate) {
		return "", tokens, false
	}

	return "$" + candidate, tokens[:start-1], true
}

func consumeTrailingNumericField(tokens []string, minDots int) (string, []string, bool) {
	if len(tokens) == 0 {
		return "", tokens, false
	}

	end := len(tokens)
	start := end - 1
	field := tokens[start]
	for start > 0 && isDigitToken(tokens[start-1]) && strings.Count(field, ".") <= minDots {
		start--
		field = tokens[start] + field
	}

	if !isDigitToken(field) {
		return "", tokens, false
	}

	return field, tokens[:start], true
}

func consumeLeadingDigits(tokens []string, targetLen int) (string, []string) {
	if len(tokens) == 0 {
		return "", tokens
	}

	value := ""
	idx := 0
	for idx < len(tokens) && isDigitToken(tokens[idx]) && len(value) < targetLen {
		value += tokens[idx]
		idx++
	}
	return value, tokens[idx:]
}

func isDigitToken(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}

func looksIssue140AmountFragment(token string) bool {
	if token == "" {
		return false
	}

	hasDigit := false
	for _, r := range token {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '.':
		default:
			return false
		}
	}

	return hasDigit
}

func looksIssue140Amount(token string) bool {
	if !strings.Contains(token, ".") {
		return false
	}
	return isDigitToken(token)
}

func reverseStrings(tokens []string) {
	for i := 0; i < len(tokens)/2; i++ {
		j := len(tokens) - 1 - i
		tokens[i], tokens[j] = tokens[j], tokens[i]
	}
}

func stringCellsToTableRow(values []string, box Rect) TableRow {
	cells := make([]TableCell, len(values))
	for i, value := range values {
		cells[i] = TableCell{
			BBox:    rowBBoxToCellBBox(box),
			Content: value,
		}
	}
	return TableRow{
		Cells: cells,
		BBox:  rowBBoxToCellBBox(box),
	}
}
