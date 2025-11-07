package pdfmarkdown

import (
	"io"
	"log"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/pkg/errors"
)

// ProcessingMetrics contains timing and statistics for PDF conversion
type ProcessingMetrics struct {
	TotalTime       time.Duration
	DocumentOpen    time.Duration
	PageExtractions []PageMetrics
	Statistics      DocumentStatistics
}

// PageMetrics contains timing for a single page
type PageMetrics struct {
	PageNumber int
	Duration   time.Duration
}

// DocumentStatistics contains document-level statistics
type DocumentStatistics struct {
	TotalPages      int
	TotalParagraphs int
	TotalTables     int
	TotalHeadings   int
	TotalWords      int
	TotalCharacters int
}

// Config controls markdown conversion behavior.
type Config struct {
	// IncludePageBreaks adds "---" separators between pages (default: true)
	IncludePageBreaks bool

	// MinHeadingFontSize is the minimum font size difference to detect headings
	// A value of 0 disables size-based heading detection (default: 1.15x body text)
	MinHeadingFontSize float64

	// DetectTables enables table detection and extraction (default: false)
	DetectTables bool

	// TableSettings configures table detection behavior (default: DefaultTableSettings())
	TableSettings TableSettings

	// UseSegmentBasedTables enables PDF-TREX segment-based table detection
	// This works better for tables without ruling lines (default: true)
	UseSegmentBasedTables bool

	// UseAdaptiveThresholds enables document-specific threshold calculation
	// Based on spacing distribution analysis (default: true)
	UseAdaptiveThresholds bool

	// EnableMetricsLogging enables processing time and statistics logging (default: false)
	EnableMetricsLogging bool
}

// DefaultConfig returns the default converter configuration.
func DefaultConfig() Config {
	return Config{
		IncludePageBreaks:      true,
		MinHeadingFontSize:     1.15,
		DetectTables:           true,
		TableSettings:          DefaultTableSettings(),
		UseSegmentBasedTables:  false, // Opt-in: good for PDFs without ruling lines
		UseAdaptiveThresholds:  true,
	}
}

// Converter converts PDFs to markdown using pdfium text extraction.
type Converter struct {
	instance pdfium.Pdfium
	config   Config
}

// NewConverter creates a new PDF to markdown converter with default configuration.
func NewConverter(instance pdfium.Pdfium) *Converter {
	return &Converter{
		instance: instance,
		config:   DefaultConfig(),
	}
}

// NewConverterWithConfig creates a new PDF to markdown converter with custom configuration.
func NewConverterWithConfig(instance pdfium.Pdfium, config Config) *Converter {
	return &Converter{
		instance: instance,
		config:   config,
	}
}

// ConvertFile converts a PDF file to markdown.
func (c *Converter) ConvertFile(filePath string) (string, error) {
	// Open the PDF document
	doc, err := c.instance.OpenDocument(&requests.OpenDocument{
		FilePath: &filePath,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to open PDF document")
	}
	defer c.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	return c.convertDocument(doc.Document)
}

// ConvertBytes converts PDF bytes to markdown.
func (c *Converter) ConvertBytes(pdfBytes []byte) (string, error) {
	// Open the PDF document
	doc, err := c.instance.OpenDocument(&requests.OpenDocument{
		File: &pdfBytes,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to open PDF document")
	}
	defer c.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	return c.convertDocument(doc.Document)
}

// ConvertReader converts a PDF from an io.ReadSeeker to markdown.
func (c *Converter) ConvertReader(reader io.ReadSeeker) (string, error) {
	// Open the PDF document
	doc, err := c.instance.OpenDocument(&requests.OpenDocument{
		FileReader: reader,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to open PDF document")
	}
	defer c.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	return c.convertDocument(doc.Document)
}

// ConvertPageRange converts a specific range of pages to markdown.
func (c *Converter) ConvertPageRange(filePath string, startPage, endPage int) (string, error) {
	// Open the PDF document
	doc, err := c.instance.OpenDocument(&requests.OpenDocument{
		FilePath: &filePath,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to open PDF document")
	}
	defer c.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	// Get page count
	pageCount, err := c.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to get page count")
	}

	// Validate range
	if startPage < 0 {
		startPage = 0
	}
	if endPage < 0 || endPage >= pageCount.PageCount {
		endPage = pageCount.PageCount - 1
	}
	if startPage > endPage {
		return "", errors.New("invalid page range: start page must be <= end page")
	}

	// Extract pages
	document := &Document{}
	for i := startPage; i <= endPage; i++ {
		page, err := c.extractPage(doc.Document, i)
		if err != nil {
			return "", errors.Wrapf(err, "failed to extract page %d", i+1)
		}
		document.Pages = append(document.Pages, *page)
	}

	return document.ToMarkdown(c.config), nil
}

// convertDocument converts a complete PDF document to markdown.
func (c *Converter) convertDocument(docRef references.FPDF_DOCUMENT) (string, error) {
	startTime := time.Now()

	// Get page count
	pageCount, err := c.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: docRef,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to get page count")
	}

	// Extract all pages with timing
	document := &Document{
		Pages: make([]Page, 0, pageCount.PageCount),
	}

	var pageMetrics []PageMetrics
	for i := 0; i < pageCount.PageCount; i++ {
		pageStart := time.Now()
		page, err := c.extractPage(docRef, i)
		pageDuration := time.Since(pageStart)

		if err != nil {
			return "", errors.Wrapf(err, "failed to extract page %d", i+1)
		}
		document.Pages = append(document.Pages, *page)

		pageMetrics = append(pageMetrics, PageMetrics{
			PageNumber: i + 1,
			Duration:   pageDuration,
		})

		if c.config.EnableMetricsLogging {
			log.Printf("Page %d/%d extracted in %v", i+1, pageCount.PageCount, pageDuration)
		}
	}

	// Calculate document statistics
	stats := calculateDocumentStatistics(document)

	totalTime := time.Since(startTime)

	// Log metrics if enabled
	if c.config.EnableMetricsLogging {
		logProcessingMetrics(ProcessingMetrics{
			TotalTime:       totalTime,
			PageExtractions: pageMetrics,
			Statistics:      stats,
		})
	}

	return document.ToMarkdown(c.config), nil
}

// extractPage extracts a single page with all its structure.
func (c *Converter) extractPage(docRef references.FPDF_DOCUMENT, pageIndex int) (*Page, error) {
	// Load the page
	pageResp, err := c.instance.FPDF_LoadPage(&requests.FPDF_LoadPage{
		Document: docRef,
		Index:    pageIndex,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to load page")
	}
	defer c.instance.FPDF_ClosePage(&requests.FPDF_ClosePage{
		Page: pageResp.Page,
	})

	// Extract page content
	page, err := ExtractPage(c.instance, pageResp.Page, pageIndex+1, c.config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract page content")
	}

	return page, nil
}

// calculateDocumentStatistics calculates statistics for the document
func calculateDocumentStatistics(doc *Document) DocumentStatistics {
	stats := DocumentStatistics{
		TotalPages: len(doc.Pages),
	}

	for _, page := range doc.Pages {
		stats.TotalParagraphs += len(page.Paragraphs)
		stats.TotalTables += len(page.Tables)

		for _, para := range page.Paragraphs {
			if para.IsHeading {
				stats.TotalHeadings++
			}

			for _, line := range para.Lines {
				stats.TotalWords += len(line.Words)
				for _, word := range line.Words {
					stats.TotalCharacters += len(word.Text)
				}
			}
		}
	}

	return stats
}

// logProcessingMetrics logs the processing metrics in a readable format
func logProcessingMetrics(metrics ProcessingMetrics) {
	log.Println("┌─────────────────────────────────────────────┐")
	log.Println("│ PDF Processing Metrics                      │")
	log.Println("├─────────────────────────────────────────────┤")
	log.Printf("│ Total Time: %-31v │\n", metrics.TotalTime.Round(time.Millisecond))
	log.Println("├─────────────────────────────────────────────┤")
	log.Println("│ Document Statistics                         │")
	log.Println("├─────────────────────────────────────────────┤")
	log.Printf("│   Pages:      %-29d │\n", metrics.Statistics.TotalPages)
	log.Printf("│   Paragraphs: %-29d │\n", metrics.Statistics.TotalParagraphs)
	log.Printf("│   Headings:   %-29d │\n", metrics.Statistics.TotalHeadings)
	log.Printf("│   Tables:     %-29d │\n", metrics.Statistics.TotalTables)
	log.Printf("│   Words:      %-29d │\n", metrics.Statistics.TotalWords)
	log.Printf("│   Characters: %-29d │\n", metrics.Statistics.TotalCharacters)
	log.Println("├─────────────────────────────────────────────┤")
	log.Println("│ Per-Page Timing                             │")
	log.Println("├─────────────────────────────────────────────┤")

	// Show timing for each page
	for _, pm := range metrics.PageExtractions {
		log.Printf("│   Page %2d: %-30v │\n", pm.PageNumber, pm.Duration.Round(time.Millisecond))
	}

	// Show average time per page
	if len(metrics.PageExtractions) > 0 {
		avgTime := metrics.TotalTime / time.Duration(len(metrics.PageExtractions))
		log.Println("├─────────────────────────────────────────────┤")
		log.Printf("│ Avg per page: %-28v │\n", avgTime.Round(time.Millisecond))
	}

	log.Println("└─────────────────────────────────────────────┘")
}

// ConvertFileWithMetrics converts a PDF and returns both markdown and metrics
func (c *Converter) ConvertFileWithMetrics(filePath string) (string, ProcessingMetrics, error) {
	startTime := time.Now()
	openStart := time.Now()

	// Open the PDF document
	doc, err := c.instance.OpenDocument(&requests.OpenDocument{
		FilePath: &filePath,
	})
	if err != nil {
		return "", ProcessingMetrics{}, errors.Wrap(err, "failed to open PDF document")
	}
	defer c.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	documentOpenTime := time.Since(openStart)

	// Get page count
	pageCount, err := c.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	if err != nil {
		return "", ProcessingMetrics{}, errors.Wrap(err, "failed to get page count")
	}

	// Extract all pages with timing
	document := &Document{
		Pages: make([]Page, 0, pageCount.PageCount),
	}

	var pageMetrics []PageMetrics
	for i := 0; i < pageCount.PageCount; i++ {
		pageStart := time.Now()
		page, err := c.extractPage(doc.Document, i)
		pageDuration := time.Since(pageStart)

		if err != nil {
			return "", ProcessingMetrics{}, errors.Wrapf(err, "failed to extract page %d", i+1)
		}
		document.Pages = append(document.Pages, *page)

		pageMetrics = append(pageMetrics, PageMetrics{
			PageNumber: i + 1,
			Duration:   pageDuration,
		})
	}

	// Calculate statistics
	stats := calculateDocumentStatistics(document)

	// Generate markdown
	markdown := document.ToMarkdown(c.config)

	totalTime := time.Since(startTime)

	metrics := ProcessingMetrics{
		TotalTime:       totalTime,
		DocumentOpen:    documentOpenTime,
		PageExtractions: pageMetrics,
		Statistics:      stats,
	}

	return markdown, metrics, nil
}

// GetDocumentInfo returns basic information about a PDF without converting it.
func (c *Converter) GetDocumentInfo(filePath string) (*DocumentInfo, error) {
	doc, err := c.instance.OpenDocument(&requests.OpenDocument{
		FilePath: &filePath,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to open PDF document")
	}
	defer c.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	pageCount, err := c.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get page count")
	}

	return &DocumentInfo{
		PageCount: pageCount.PageCount,
	}, nil
}

// DocumentInfo contains basic information about a PDF document.
type DocumentInfo struct {
	PageCount int
}
