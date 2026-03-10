# Issue 192 Suspicious Encoding Recovery Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a local suspicious-encoding recovery stage that suppresses low-confidence gibberish while preserving readable anchors in PDFs like `issue-192`.

**Architecture:** Reuse the existing text-block pipeline. Score each `TextBlock` for encoding corruption, preserve high-confidence tokens such as numbers and readable ASCII words, suppress low-confidence noisy runs, and keep the rest of the extraction flow unchanged.

**Tech Stack:** Go, pdfium extraction pipeline, existing `TextBlock` / `Line` / `EnrichedWord` structures, Go test suite.

---

### Task 1: Add failing regressions for suspicious encoding suppression

**Files:**
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/financial_reports_test.go`
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/edge_cases_test.go`

**Step 1: Write the failing test**

- Add a synthetic block test where a noisy `A/a/b` run surrounds a numeric or code anchor, and only the anchor should survive.
- Add a synthetic healthy-text test to prove normal readable words are not suppressed.
- Tighten the `issue-192` edge-case test to assert reduced noise instead of accepting any non-empty markdown.

**Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestRecoverSuspiciousEncodingBlock|TestRecoverSuspiciousEncodingBlock_LeavesHealthyText|TestEdgeCases_VerticalText' -count=1`

Expected: FAIL because the suspicious-encoding recovery does not exist yet.

### Task 2: Implement suspicious encoding scoring and suppression

**Files:**
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/rotated_block_recovery.go`
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/structure.go`

**Step 1: Write minimal implementation**

- Add helpers for:
  - encoding-corruption scoring,
  - high-confidence token preservation,
  - low-confidence noisy token suppression,
  - line/block rebuilding after suppression.
- Integrate the stage into the existing `recoverSuspiciousTextBlocks` flow.

**Step 2: Run focused tests**

Run: `go test ./... -run 'TestRecoverSuspiciousEncodingBlock|TestRecoverSuspiciousEncodingBlock_LeavesHealthyText|TestEdgeCases_VerticalText' -count=1`

Expected: PASS

### Task 3: Verify interaction with existing suspicious-block recovery

**Files:**
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/issue_140_improved_test.go`

**Step 1: Run focused compatibility tests**

Run: `go test ./... -run 'TestIssue140_DefaultRenderingProducesReadableTable|TestIssue140_ImprovedTableDetection' -count=1`

Expected: PASS

### Task 4: Full verification and cleanup

**Files:**
- Modify: none required

**Step 1: Run the full suite**

Run: `go test ./... -count=1`

Expected: PASS

**Step 2: Restore generated artifacts**

Run: `git restore testdata/issue-140-output.md testdata/issue-140-table-analysis.json`

Expected: generated analysis artifacts are not included in the change set.
