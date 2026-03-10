# Suspicious Block Recovery Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Generalize local garbled-text recovery so suspicious text blocks are normalized into more readable text before paragraph and table extraction.

**Architecture:** Replace the narrow `issue-140` block normalizer with a generic suspicious-block recovery stage. Score each `TextBlock`, generate a small set of local reconstruction candidates for suspicious blocks, and keep the best candidate only when it clearly improves readability.

**Tech Stack:** Go, pdfium-based PDF extraction pipeline, existing `TextBlock` / `Line` / `EnrichedWord` structures, Go test suite.

---

### Task 1: Add failing regression tests for generalized suspicious-block recovery

**Files:**
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/financial_reports_test.go`
- Test: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/issue_140_improved_test.go`

**Step 1: Write the failing test**

- Add a synthetic test for a suspicious local block with reversed token order that should normalize into readable text.
- Add a synthetic test for numeric-fragment stitching such as `253. 1 5`.
- Keep the existing conservative non-trigger test.

**Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestRecoverSuspiciousTextBlock|TestRecoverSuspiciousTextBlock_LeavesHealthyBlockAlone|TestIssue140_DefaultRenderingProducesReadableTable' -count=1`

Expected: FAIL because the generalized suspicious-block recovery does not exist yet.

### Task 2: Replace the narrow block normalizer with a generic suspicious-block recovery stage

**Files:**
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/structure.go`
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/rotated_block_recovery.go`

**Step 1: Write minimal implementation**

- Rename the current `issue-140` block hook into a generic recovery entry point.
- Add garble scoring helpers for single-character density, fragmented numeric tokens, and poor lexical quality.
- Add candidate generation for:
  - current order with smarter stitching,
  - rotation-normalized horizontal regrouping,
  - reversed token order within lines.
- Add readability scoring and only accept a replacement when it clearly improves the score.

**Step 2: Run focused tests**

Run: `go test ./... -run 'TestRecoverSuspiciousTextBlock|TestRecoverSuspiciousTextBlock_LeavesHealthyBlockAlone|TestIssue140_DefaultRenderingProducesReadableTable' -count=1`

Expected: PASS

### Task 3: Keep issue-140 table recovery compatible with the generalized text repair

**Files:**
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/issue140_table_recovery.go`
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/extract.go`

**Step 1: Adjust minimal implementation only if needed**

- Keep the special recovered table path gated.
- Ensure it still works with the generalized normalized lines and token stitching.

**Step 2: Run focused render verification**

Run: `go run ./cmd/pdfmarkdown --input '/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/testdata/issue-140-example.pdf'`

Expected: readable headings and a 4-row top table.

### Task 4: Full verification and cleanup

**Files:**
- Modify: none required

**Step 1: Run the full suite**

Run: `go test ./... -count=1`

Expected: PASS

**Step 2: Restore generated artifacts**

Run: `git restore testdata/issue-140-output.md testdata/issue-140-table-analysis.json`

Expected: generated analysis artifacts are not included in the change set.
