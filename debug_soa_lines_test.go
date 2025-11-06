package pdfmarkdown_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"

	pdfmarkdown "github.com/Alcova-AI/pdfmarkdown"
)

func TestDebug_SOA_Lines(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	pdfPath := filepath.Join("testdata", "Mock Statement of Advice.pdf")
	doc, err := instance.OpenDocument(&requests.OpenDocument{
		FilePath: &pdfPath,
	})
	require.NoError(t, err)
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	// Test first page
	pageResp, err := instance.FPDF_LoadPage(&requests.FPDF_LoadPage{
		Document: doc.Document,
		Index:    0,
	})
	require.NoError(t, err)
	defer instance.FPDF_ClosePage(&requests.FPDF_ClosePage{
		Page: pageResp.Page,
	})

	config := pdfmarkdown.DefaultConfig()
	config.DetectTables = true

	page, err := pdfmarkdown.ExtractPage(instance, pageResp.Page, 1, config)
	require.NoError(t, err)

	fmt.Printf("\n=== SOA Page 1 ===\n")
	fmt.Printf("Extracted lines: %d\n", len(page.Lines))
	fmt.Printf("Tables detected: %d\n", len(page.Tables))

	if len(page.Lines) > 0 {
		fmt.Printf("\nFirst 10 lines:\n")
		count := 10
		if len(page.Lines) < count {
			count = len(page.Lines)
		}
		for i := 0; i < count; i++ {
			line := page.Lines[i]
			fmt.Printf("  %s: (%.2f, %.2f) -> (%.2f, %.2f)\n",
				line.Orientation, line.X0, line.Top, line.X1, line.Bottom)
		}
	}
}
