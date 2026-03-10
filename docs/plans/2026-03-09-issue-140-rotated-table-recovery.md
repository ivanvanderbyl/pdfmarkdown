# Issue-140 Rotated Table Recovery Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Recover readable text and a real markdown table from `testdata/issue-140-example.pdf` without changing behavior for normal tables.

**Architecture:** Add a narrowly gated rotated-text recovery path during paragraph extraction. When a page is dominated by 270° single-character words and normal extraction would produce column-like garbage, normalize the rotated block into horizontal words and lines before table detection runs. Existing table support remains the default path and only sees normalized content for pages that strongly match this failure pattern.

**Tech Stack:** Go, existing word extraction, rotation grouping, paragraph construction, segment-based and line-based table detection.

---

### Task 1: Lock down issue-140 rendering expectations

**Files:**
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/issue_140_improved_test.go`

**Step 1: Write the failing test**
- Add a regression asserting default markdown output contains a readable top table header and a real row from the purchase-order table.
- Assert the output no longer contains the current spaced-letter garble.

**Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestIssue140_DefaultRenderingProducesReadableTable' -count=1`

Expected: FAIL because current output is still garbled.

**Step 3: Write minimal implementation**
- Add a gated rotated-block normalization helper.
- Thread normalized words/lines through paragraph building so table detection sees recovered rows.

**Step 4: Run test to verify it passes**

Run: `go test ./... -run 'TestIssue140_DefaultRenderingProducesReadableTable' -count=1`

Expected: PASS.

### Task 2: Keep the recovery path narrowly gated

**Files:**
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/financial_reports_test.go`
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/rotation.go`
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/structure.go`

**Step 1: Write the failing guard test**
- Add a synthetic test proving ordinary rotated/vertical text does not trigger the issue-140 recovery path unless the page is dominated by single-character rotated content.

**Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestNormalizeRotatedBlock_RequiresIssue140Signals' -count=1`

Expected: FAIL because the gate does not exist yet.

**Step 3: Write minimal implementation**
- Add a strong-evidence predicate for issue-140-style rotated blocks.
- Apply normalization only when the predicate passes.

**Step 4: Run test to verify it passes**

Run: `go test ./... -run 'TestNormalizeRotatedBlock_RequiresIssue140Signals' -count=1`

Expected: PASS.

### Task 3: Verify the whole extractor still passes

**Files:**
- No committed file changes required.

**Step 1: Run focused issue-140 tests**

Run: `go test ./... -run 'TestIssue140_DefaultRenderingProducesReadableTable|TestNormalizeRotatedBlock_RequiresIssue140Signals' -count=1`

Expected: PASS.

**Step 2: Run full suite**

Run: `go test ./... -count=1`

Expected: PASS.

**Step 3: Manually render the sample**

Run: `go run ./cmd/pdfmarkdown --input '/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/testdata/issue-140-example.pdf'`

Expected:
- readable table header like `Line no | UPC code | Location code`,
- readable first row with `0085648100305`,
- readable section headings like `Associated claims`,
- no spaced-letter garble.
