# Bank Statement Ledger Extraction Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extract bank-statement ledgers as clean markdown tables that preserve explicit PDF columns, keep balance/boundary rows, and exclude non-ledger sections.

**Architecture:** Add a statement-ledger detection pass after paragraph construction and before final table rendering. Detect ledger regions from line-level date/amount patterns, infer or preserve ledger columns, build logical rows with continuation handling, and replace generic tables only when the ledger signal is strong.

**Tech Stack:** Go, existing paragraph/line extraction, existing table and markdown rendering pipeline.

---

### Task 1: Lock down ledger expectations with tests

**Files:**
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/financial_reports_test.go`

**Step 1: Write the failing tests**
- Add a synthetic statement-page test proving explicit ledger columns are preserved.
- Add a synthetic statement-page test proving continuation text stays in the owning row and footer/promotional text is excluded.

**Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestDetectLedgerTable_PreservesExplicitColumns|TestDetectLedgerTable_MergesContinuationRowsAndExcludesNonLedger' -count=1`

Expected: FAIL because ledger-specific extraction does not exist yet.

**Step 3: Write minimal implementation**
- Add a ledger detection helper that operates on page lines.
- Add a ledger row builder that groups continuation lines into the owning row.

**Step 4: Run test to verify it passes**

Run: `go test ./... -run 'TestDetectLedgerTable_PreservesExplicitColumns|TestDetectLedgerTable_MergesContinuationRowsAndExcludesNonLedger' -count=1`

Expected: PASS.

### Task 2: Integrate ledger detection into page extraction

**Files:**
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/extract.go`
- Modify: `/Users/ivanvanderbyl/dev/Alcova-AI/pdf-markdown/markdown.go`

**Step 1: Write the failing integration test**
- Add a document-level test proving ledger tables replace generic noisy tables when the page strongly matches a bank statement.

**Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestDocumentToMarkdown_PrefersDetectedLedgerTableForBankStatementPage' -count=1`

Expected: FAIL because rendering still prefers generic tables.

**Step 3: Write minimal implementation**
- Add ledger detection to page construction.
- Prefer detected ledger tables over generic tables only for strong statement pages.

**Step 4: Run test to verify it passes**

Run: `go test ./... -run 'TestDocumentToMarkdown_PrefersDetectedLedgerTableForBankStatementPage' -count=1`

Expected: PASS.

### Task 3: Validate against the supplied statement PDF

**Files:**
- No committed file changes required.

**Step 1: Run full test suite**

Run: `go test ./... -count=1`

Expected: PASS.

**Step 2: Run converter on the supplied statement PDF**

Run: `go run ./cmd/pdfmarkdown --input '/Users/ivanvanderbyl/Downloads/StreamlineStatement28Jan260123602002865990004MISSAVDANIELSANDMRISVANDERBYL.PDF' --start-page 0 --end-page 2`

Expected:
- transaction/balance rows render as a ledger table,
- explicit ledger columns survive,
- marketing copy and authority sections are outside the ledger table.
