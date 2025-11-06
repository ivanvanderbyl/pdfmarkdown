package pdfmarkdown

import (
	"io"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/pkg/errors"
)

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
}

// DefaultConfig returns the default converter configuration.
func DefaultConfig() Config {
	return Config{
		IncludePageBreaks:      true,
		MinHeadingFontSize:     1.15,
		DetectTables:           true,
		TableSettings:          DefaultTableSettings(),
		UseSegmentBasedTables:  true,
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
	// Get page count
	pageCount, err := c.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: docRef,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to get page count")
	}

	// Extract all pages
	document := &Document{
		Pages: make([]Page, 0, pageCount.PageCount),
	}

	for i := 0; i < pageCount.PageCount; i++ {
		page, err := c.extractPage(docRef, i)
		if err != nil {
			return "", errors.Wrapf(err, "failed to extract page %d", i+1)
		}
		document.Pages = append(document.Pages, *page)
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
