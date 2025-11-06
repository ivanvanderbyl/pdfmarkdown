package pdfmarkdown

import (
	"fmt"
	"strings"
)

// ToMarkdown converts a document to markdown format.
func (d *Document) ToMarkdown(config Config) string {
	var sb strings.Builder

	for i, page := range d.Pages {
		if i > 0 && config.IncludePageBreaks {
			sb.WriteString("\n\n---\n\n") // Page break
		}

		for j, para := range page.Paragraphs {
			// Add paragraph separator
			if j > 0 {
				sb.WriteString("\n\n")
			}

			// Convert paragraph to markdown
			paraText := convertParagraphToMarkdown(para)
			sb.WriteString(paraText)
		}

		// Add tables at the end of the page content
		if config.DetectTables && len(page.Tables) > 0 {
			for _, table := range page.Tables {
				if len(page.Paragraphs) > 0 || len(page.Tables) > 1 {
					sb.WriteString("\n\n")
				}
				tableText := convertTableToMarkdown(table)
				sb.WriteString(tableText)
			}
		}
	}

	return sb.String()
}

// convertParagraphToMarkdown converts a single paragraph to markdown.
func convertParagraphToMarkdown(para Paragraph) string {
	if len(para.Lines) == 0 {
		return ""
	}

	// Handle headings
	if para.IsHeading {
		prefix := strings.Repeat("#", para.HeadingLevel)
		text := strings.TrimRight(para.Text(), " \t")
		return fmt.Sprintf("%s %s", prefix, text)
	}

	// Handle code blocks
	if para.IsCode {
		lines := strings.Split(para.Text(), "\n")
		var sb strings.Builder
		sb.WriteString("```\n")
		for _, line := range lines {
			// Trim trailing whitespace from code lines
			sb.WriteString(strings.TrimRight(line, " \t"))
			sb.WriteString("\n")
		}
		sb.WriteString("```")
		return sb.String()
	}

	// Handle lists
	if para.IsList {
		text := strings.TrimRight(para.Text(), " \t")
		// Ensure proper list formatting
		if !strings.HasPrefix(text, "* ") &&
			!strings.HasPrefix(text, "- ") &&
			!strings.HasPrefix(text, "+ ") {
			// If it starts with a number, keep it, otherwise add bullet
			if len(text) > 0 && (text[0] >= '0' && text[0] <= '9') {
				// Numbered list - keep as is
				return text
			}
			return "* " + text
		}
		return text
	}

	// Handle regular paragraphs with inline formatting
	var sb strings.Builder
	for i, line := range para.Lines {
		if i > 0 {
			// Preserve line breaks within paragraphs using Markdown hard line break
			// This is important for structured content like key-value pairs,
			// lists, and tabular data that span multiple lines
			sb.WriteString("  \n")
		}

		// Build the line content
		var lineSb strings.Builder
		for j, word := range line.Words {
			if j > 0 {
				lineSb.WriteString(" ")
			}

			// Apply inline formatting
			formattedWord := applyInlineFormatting(word)
			lineSb.WriteString(formattedWord)
		}

		// Trim trailing whitespace from the line before adding to paragraph
		lineText := strings.TrimRight(lineSb.String(), " \t")
		sb.WriteString(lineText)
	}

	return sb.String()
}

// applyInlineFormatting applies markdown formatting to a word based on its style.
func applyInlineFormatting(word EnrichedWord) string {
	text := word.Text

	// Apply bold
	if word.IsBold && word.IsItalic {
		return fmt.Sprintf("***%s***", text)
	}

	if word.IsBold {
		return fmt.Sprintf("**%s**", text)
	}

	// Apply italic
	if word.IsItalic {
		return fmt.Sprintf("*%s*", text)
	}

	// Apply code (monospace)
	if word.IsMonospace {
		return fmt.Sprintf("`%s`", text)
	}

	return text
}

// convertTableToMarkdown converts a table to markdown format.
func convertTableToMarkdown(table Table) string {
	if len(table.Rows) == 0 {
		return ""
	}

	var sb strings.Builder

	// Calculate column widths based on content
	colWidths := make([]int, table.NumCols)
	for _, row := range table.Rows {
		for colIdx, cell := range row.Cells {
			if colIdx < len(colWidths) {
				contentLen := len(cell.Content)
				if contentLen > colWidths[colIdx] {
					colWidths[colIdx] = contentLen
				}
			}
		}
	}

	// Ensure minimum width of 3 for each column
	for i := range colWidths {
		if colWidths[i] < 3 {
			colWidths[i] = 3
		}
	}

	// Write rows
	for rowIdx, row := range table.Rows {
		// Write cell content
		sb.WriteString("|")
		for colIdx := 0; colIdx < table.NumCols; colIdx++ {
			content := ""
			if colIdx < len(row.Cells) {
				content = strings.ReplaceAll(row.Cells[colIdx].Content, "\n", " ")
			}
			// Pad content to column width
			padding := colWidths[colIdx] - len(content)
			sb.WriteString(" ")
			sb.WriteString(content)
			sb.WriteString(strings.Repeat(" ", padding))
			sb.WriteString(" |")
		}
		sb.WriteString("\n")

		// Add separator row after first row (header)
		if rowIdx == 0 {
			sb.WriteString("|")
			for colIdx := 0; colIdx < table.NumCols; colIdx++ {
				sb.WriteString(strings.Repeat("-", colWidths[colIdx]+2))
				sb.WriteString("|")
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// PageToMarkdown converts a single page to markdown.
func (p *Page) ToMarkdown() string {
	var sb strings.Builder

	for i, para := range p.Paragraphs {
		if i > 0 {
			sb.WriteString("\n\n")
		}

		paraText := convertParagraphToMarkdown(para)
		sb.WriteString(paraText)
	}

	// Add tables at the end of the page content
	if len(p.Tables) > 0 {
		for _, table := range p.Tables {
			if len(p.Paragraphs) > 0 || len(p.Tables) > 1 {
				sb.WriteString("\n\n")
			}
			tableText := convertTableToMarkdown(table)
			sb.WriteString(tableText)
		}
	}

	return sb.String()
}
