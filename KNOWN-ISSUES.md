# Known Issues & Limitations

## Issue #140: Word Boundary Detection in PDFs Without Whitespace

### Problem Description

Some PDFs have characters placed without explicit whitespace characters between words. This causes the parser to concatenate all text into single "words", resulting in output like:

```
"numberPORateHandlingAmountAccruedAmountBillQuantityItemDescription..."
```

Instead of:

```
"number PO Rate Handling Amount Accrued Amount Bill Quantity Item Description..."
```

### Affected Document Type

**Example:** `testdata/issue-140-example.pdf`

This document contains a purchase order table that is:
1. **Rotated 90 degrees** (landscape orientation)
2. **No whitespace between words** in the PDF character stream
3. **Multiple distinct words** visually separated but not in PDF encoding

### Current Behavior

**What happens:**
- Characters are grouped into words based on whitespace (extract.go:227)
- No whitespace → entire line becomes one "word"
- `mergeCloseWords()` further merges words with gaps < 2px
- Result: Concatenated text without spaces

**Example output:**
```markdown
| numberPORateHandlingAmount... | 0.0000388.57$0.61$637CHOC... |
```

**What it should be:**
```markdown
| Line no | UPC code | Location code | Item Description | Quantity | Bill Amount | Accrued Amount | Handling Rate | PO number |
|---------|----------|---------------|------------------|----------|-------------|----------------|---------------|-----------|
| 5       | 00856... | LILYSKMACE... | CHOC ALMND SL... | 637      | $0.61       | $388.57        | 0.0000        |           |
```

### Root Cause

**PDF Structure:**
```
Character stream: ['n','u','m','b','e','r','P','O','R','a','t','e',...]
Positions:        [x:0][x:3][x:6][x:9][x:12][x:15][x:18][x:21][x:24]...
Whitespace:       NONE
```

**Our Algorithm:**
1. `groupCharsIntoWords()` only splits on `' '`, `'\t'`, `'\n'`, `'\r'`
2. If no whitespace chars → all chars become one word
3. `mergeCloseWords()` then merges words closer than 2px

### Implemented Solutions

#### ✅ Solution 1: Rotation-Aware Word Boundary Detection (IMPLEMENTED)

**Status:** Partially implemented in extract.go

**What was implemented:**
1. **Rotation detection** - Detects text rotation angle (in radians) and converts to degrees
2. **Y-axis gap analysis for rotated text** - For 90°/270° rotated text, analyzes vertical gaps instead of horizontal gaps
3. **Character reversal for 270° text** - Automatically reverses character order for bottom-to-top text
4. **Case transition splitting** - Splits on lowercase→uppercase transitions (camelCase)
5. **Digit/letter transitions** - Splits between digits and letters
6. **Currency/punctuation boundaries** - Treats special characters as word boundaries

**Implementation location:** `extract.go:327-600`

**Functions added:**
- `isRotatedText(angle float32) bool` - Detects if text is rotated (not 0° or 180°)
- `shouldReverseCharOrder(angle float32) bool` - Determines if characters should be reversed (for 270° text)
- `detectWordBoundariesRotationAware(chars []EnrichedChar) []int` - Main boundary detection logic with Y-axis gap analysis
- `detectWordBoundaries(chars []EnrichedChar) []int` - Conservative detection for normal text (whitespace + special chars only)
- `isLowerCase`, `isUpperCase`, `isDigit`, `isAlpha`, `isCurrency`, `isPunctuation` - Character classification helpers
- `calculateAverageCharWidth(chars []EnrichedChar) float64` - Statistical analysis

**Design decision:** Gap-based and heuristic word boundary detection is ONLY applied to rotated text. Normal horizontal text relies on explicit whitespace to avoid false positives (like splitting "STATEMENT" into "STATE M E N T").

**Results on issue-140-example.pdf:**
- ✅ Numbers now properly separated: "0000 .075 .883$16 .0$736" (was: "0.0000388.57$0.61$637")
- ✅ Rotation detected correctly (270° / 4.71 radians)
- ⚠️ Text still backwards due to PDF rendering order: "rebmun" instead of "number"
- ⚠️ ALL-CAPS words without gaps still concatenated: "COHCDNMLADTLS" (requires dictionary-based segmentation)

### Remaining Limitations

#### Limitation 1: Character Order in Rotated Text

**Problem:** 270° rotated text is rendered bottom-to-top in PDFs, resulting in reversed character order.

**Example:** "UPC" appears as "CPU", "number" appears as "rebmun"

**Why it's hard to fix:** PDFs store characters in the order they appear in the document stream, not visual reading order. Reversing at the character level works, but the rotation.go module may reverse again at the line level, causing double-reversal.

**Potential solutions:**
1. Coordinate with rotation.go to avoid double-reversal
2. Handle reversal at a different pipeline stage
3. Use visual coordinate sorting instead of document order

#### Limitation 2: ALL-CAPS Words Without Gaps

**Problem:** Words in ALL CAPS with no physical spacing cannot be split using heuristic methods.

**Example:** "COHCDNMLADTLS" should be "CHOC ALMND SLTD"

**Why gap analysis doesn't work:** The PDF truly has no gaps - characters are placed at evenly-spaced positions with no extra spacing between words.

**Why case transitions don't work:** All characters are uppercase, so there are no case boundaries.

**Requires advanced solutions:**

### Proposed Advanced Solutions

#### Solution 2: Dictionary-Based Word Segmentation (Priority: MEDIUM)

```go
func detectWordBoundaries(chars []EnrichedChar) []int {
    var boundaries []int

    for i := 1; i < len(chars); i++ {
        prev, curr := chars[i-1], chars[i]

        // Detect boundaries by:

        // 1. Gap analysis: Gaps > 0.3 * avgCharWidth
        gap := curr.Box.X0 - prev.Box.X1
        avgWidth := (prev.Box.Width() + curr.Box.Width()) / 2
        if gap > avgWidth * 0.3 {
            boundaries = append(boundaries, i)
            continue
        }

        // 2. Case transitions: lowercase → uppercase (camelCase)
        if isLower(prev.Text) && isUpper(curr.Text) {
            boundaries = append(boundaries, i)
            continue
        }

        // 3. Digit/letter transitions
        if isDigit(prev.Text) && isAlpha(curr.Text) {
            boundaries = append(boundaries, i)
            continue
        }

        // 4. Special characters (currency, punctuation)
        if isCurrency(curr.Text) || isPunctuation(curr.Text) {
            boundaries = append(boundaries, i)
            continue
        }
    }

    return boundaries
}
```

**Impact:** Would correctly split concatenated text
**Complexity:** Medium (requires careful tuning to avoid over-splitting)
**Files to modify:** extract.go

---

#### Solution 2: Statistical Gap Analysis (Priority: MEDIUM)

Calculate average character width and gap distribution, then use statistical threshold:

```go
func calculateAverageCharWidth(chars []EnrichedChar) float64 {
    var totalWidth float64
    for _, char := range chars {
        totalWidth += char.Box.Width()
    }
    return totalWidth / float64(len(chars))
}

func groupCharsIntoWordsAdvanced(chars []EnrichedChar) []EnrichedWord {
    avgCharWidth := calculateAverageCharWidth(chars)
    wordBoundaryThreshold := avgCharWidth * 0.4  // Adaptive threshold

    // Use threshold to detect word boundaries
    // ...
}
```

**Impact:** Document-adaptive word splitting
**Complexity:** Low
**Files to modify:** extract.go

---

#### Solution 3: Post-Processing with NLP (Priority: LOW)

Use dictionary/language model to insert spaces:

```go
func insertSpaces(concatenatedText string) string {
    // Use dictionary or language model to find likely word boundaries
    // This is computationally expensive and requires external dependencies
    return spacedText
}
```

**Impact:** High accuracy but high complexity
**Complexity:** Very High (requires NLP library)
**Not recommended:** Adds heavy dependencies

---

### Workaround for Users

For PDFs with this issue, users can:

1. **Pre-process PDF:** Use PDF editing tools to add spacing
2. **OCR re-rendering:** Convert PDF to images and OCR back
3. **Custom post-processing:** Parse the concatenated output and split manually
4. **Use segment-based detection:** Sometimes handles these better

---

### Related Issues

- **Rotation handling:** Partially addressed by rotation.go but needs enhancement
- **Table orientation:** 90° rotated tables appear transposed
- **mergeCloseWords threshold:** 2px may be too aggressive for some fonts

---

### Test Coverage

**Current tests:**
- `TestIssue140_ImprovedTableDetection`: Validates content extraction
- `TestIssue140_ExpectedStructure`: Documents ideal output
- `TestIssue140_KnownLimitations`: Documents limitations
- `TestIssue140_WordExtraction`: Low-level word analysis
- `TestIssue140_Analysis`: Detailed table structure analysis

**Test approach:**
- Validate key content is present (UPC codes, amounts, product names)
- Accept that structure may be imperfect
- Document expected vs actual for future improvements

---

### Future Work

Priority ranking for fixing this issue:

1. **HIGH**: Implement gap-based word boundary detection
2. **MEDIUM**: Add case-transition splitting (camelCase)
3. **MEDIUM**: Improve rotation handling for 90° tables
4. **LOW**: Add configurable mergeCloseWords threshold
5. **LOW**: Dictionary-based space insertion

---

**Issue Status:** PARTIALLY FIXED
**What was fixed:**
- ✅ Rotation-aware word boundary detection
- ✅ Gap-based splitting (works for numbers and spaced text)
- ✅ Case transition and digit/letter boundary detection
**Remaining limitations:**
- ⚠️ Backwards text in 270° rotated PDFs
- ⚠️ ALL-CAPS words without physical gaps
**Workaround Available:** Yes (pre-processing or manual post-processing)
**Future improvements:** Dictionary-based segmentation, coordinate-based text ordering
**Test Coverage:** Comprehensive (8 tests including gap analysis and angle detection)
