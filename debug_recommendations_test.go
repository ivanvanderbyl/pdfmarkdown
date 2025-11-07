package pdfmarkdown_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	pdfmarkdown "github.com/Alcova-AI/pdfmarkdown"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/stretchr/testify/require"
)

// TestDebug_RecommendationsSection analyzes the recommendations section structure
func TestDebug_RecommendationsSection(t *testing.T) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	require.NoError(t, err)
	defer pool.Close()

	instance, err := pool.GetInstance(time.Second * 30)
	require.NoError(t, err)

	converter := pdfmarkdown.NewConverter(instance)

	pdfPath := filepath.Join("testdata", "Mock Statement of Advice.pdf")
	markdown, err := converter.ConvertFile(pdfPath)
	if err != nil {
		t.Skip("Mock Statement of Advice.pdf not found")
	}
	require.NotEmpty(t, markdown)

	// Parse to find recommendations section
	lines := strings.Split(markdown, "\n")

	t.Logf("\n=== RECOMMENDATIONS SECTION ANALYSIS ===\n")

	inRecommendations := false
	lineNum := 0
	for i, line := range lines {
		if strings.Contains(line, "RECOMMENDATIONS") {
			inRecommendations = true
			lineNum = i
			t.Logf("Found RECOMMENDATIONS at line %d", i)
			continue
		}

		if inRecommendations {
			// Show next 40 lines
			if i-lineNum <= 40 {
				prefix := "  "
				if strings.HasPrefix(line, "###") {
					prefix = "H3"
				} else if strings.HasPrefix(line, "##") {
					prefix = "H2"
				} else if strings.HasPrefix(line, "#") {
					prefix = "H1"
				} else if line == "" {
					prefix = "[]"
				} else if strings.HasPrefix(line, "1.") || strings.HasPrefix(line, "2.") ||
					strings.HasPrefix(line, "3.") || strings.HasPrefix(line, "4.") {
					prefix = "**"
				}

				t.Logf("%s L%d: %s", prefix, i, line)
			} else {
				break
			}
		}
	}

	t.Logf("\n=== ANALYSIS ===")
	t.Logf("Lines starting with '2.', '3.', '4.' should be separate headings or have blank lines before them")
}
