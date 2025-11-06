# pdfmarkdown

Fast PDF to Markdown conversion using pdfium text extraction with layout and style analysis.

## Features

- **Fast extraction**: Uses native pdfium for text extraction (orders of magnitude faster than LLM processing)
- **Rich metadata**: Extracts font size, weight, style, colour, and positioning information
- **Intelligent structure detection**:
  - Headings (H1-H6) based on font size and weight
  - Paragraphs with proper line breaking
  - Bullet and numbered lists
  - Code blocks (monospace font detection)
  - Bold and italic inline formatting
  - Text alignment (left, center, right)
- **Page-aware**: Handles multi-page documents with page separators
- **Flexible API**: Convert from file path, bytes, or io.ReadSeeker

## Architecture

The converter works in three stages:

1. **Extraction**: Extract all characters with rich metadata (font, size, position, colour)
2. **Structure Analysis**: Group characters → words → lines → paragraphs, detect document structure
3. **Markdown Conversion**: Convert structured document to clean markdown

## Usage

### Basic Conversion

```go
import (
    "github.com/klippa-app/go-pdfium/webassembly"
    "github.com/Alcova-AI/pdfmarkdown"
)

// Initialise pdfium
pool, _ := webassembly.Init(webassembly.Config{
    MinIdle:  1,
    MaxIdle:  1,
    MaxTotal: 1,
})
defer pool.Close()

instance, _ := pool.GetInstance(time.Second * 30)

// Create converter
converter := pdfmarkdown.NewConverter(instance)

// Convert file
markdown, err := converter.ConvertFile("document.pdf")
if err != nil {
    log.Fatal(err)
}

fmt.Println(markdown)
```

### Convert from Bytes

```go
pdfBytes, _ := os.ReadFile("document.pdf")
markdown, err := converter.ConvertBytes(pdfBytes)
```

### Convert Specific Pages

```go
// Convert pages 0-4 (first 5 pages)
markdown, err := converter.ConvertPageRange("document.pdf", 0, 4)
```

### Get Document Info

```go
info, err := converter.GetDocumentInfo("document.pdf")
fmt.Printf("Document has %d pages\n", info.PageCount)
```

## Command Line Tool

A CLI tool is provided in the `example` directory:

```bash
cd pkg/pdfmarkdown/example
go run main.go -i input.pdf -o output.md

# Convert specific pages
go run main.go -i input.pdf -o output.md --start-page 0 --end-page 4

# Output to stdout
go run main.go -i input.pdf
```

## Markdown Features

### Headings

Headings are detected based on:
- Font size significantly larger than body text
- Bold font weight
- Single-line paragraphs

```markdown
# Large Heading (H1)
## Medium Heading (H2)
### Smaller Heading (H3)
```

### Lists

Bullet and numbered lists are detected:

```markdown
* First item
* Second item

1. Numbered item
2. Another item
```

### Inline Formatting

Bold, italic, and code are preserved:

```markdown
This is **bold** text and *italic* text with `code`.
```

### Code Blocks

Monospace paragraphs are converted to code blocks:

```markdown
```
func main() {
    fmt.Println("Hello")
}
```
```

### Page Breaks

Multi-page documents include page separators:

```markdown
Content from page 1

---

Content from page 2
```

## When to Use

**Use pdfmarkdown when:**
- PDFs have extractable text (not scanned images)
- You need fast conversion without LLM API costs
- Document structure is relatively standard
- You want to preserve formatting (bold, italic, headings)

**Fall back to LLM processing when:**
- PDFs are scanned images requiring OCR
- Complex multi-column layouts
- Tables with intricate formatting
- Semantic analysis is required

## Integration with LLM Pipeline

This package is designed to be a fast first pass before LLM processing:

```go
// Try fast extraction first
markdown, err := converter.ConvertFile(pdfPath)
if err != nil || len(markdown) < 100 {
    // Fall back to LLM-based extraction
    return llmExtractor.Extract(pdfPath)
}

// Use extracted markdown as LLM context
return llmAnalyzer.Analyze(markdown)
```

## Performance

Typical conversion speeds (varies by PDF complexity):
- Simple text PDF: ~10-50ms per page
- Complex formatted PDF: ~50-200ms per page

Compare to LLM-based extraction:
- LLM API call: ~1-5 seconds per page
- Cost: $0 vs API costs

## Limitations

- No OCR support (requires extractable text in PDF)
- Tables are not detected as structured tables
- Complex multi-column layouts may not preserve reading order perfectly
- Hyperlinks are not currently extracted (can be added)
- Images are not extracted (text only)

## Future Enhancements

- [ ] Table detection and markdown table formatting
- [ ] Hyperlink extraction
- [ ] Multi-column layout detection
- [ ] Image placeholder insertion
- [ ] Configurable style detection thresholds
- [ ] Custom markdown formatting options
