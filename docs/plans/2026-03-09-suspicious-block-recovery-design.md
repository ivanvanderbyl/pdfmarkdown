# Suspicious Block Recovery Design

## Goal

Generalize the current `issue-140` rotated-text recovery into a local-block recovery stage that prioritizes readable text for garbled PDFs without destabilizing healthy pages.

## Constraints

- Recovery must be local to suspicious blocks, not page-wide.
- Readable text is the primary goal; table reconstruction is secondary.
- Existing table support must remain the default path for healthy pages.
- The solution must not key off `issue-140` content or file names.

## Current Pipeline

- `detectTextRotation` groups words into `TextBlock`s by dominant angle.
- `buildParagraphs` and `buildParagraphsNoDetection` currently run a narrow `normalizeIssue140RotatedBlocks` hook before `mergeCloseWords`.
- `preferRecoveredIssue140Tables` adds a special recovered table for the top block after page extraction.

## Proposed Architecture

Replace the current narrow rotated-block hook with a generic suspicious-block recovery stage:

1. Score each `TextBlock` for garble signals.
2. Leave low-score blocks untouched.
3. Generate a small set of candidate reconstructions for high-score blocks.
4. Score each candidate for readability.
5. Accept the best candidate only if it clearly beats the original.

The output remains a normal `TextBlock`, so paragraph and table code keep operating on familiar structures.

## Garble Signals

- High single-character token density.
- Frequent split numeric/currency fragments such as `41 9.68`, `253. 1 5`, `0 . 0 0 0 0`.
- Low lexical quality in very long lines.
- Reversed-looking token order inside lines.
- Rotation or reading-direction disagreement with more readable alternate orderings.

## Candidate Reconstructions

- Original block with smarter local token stitching.
- Rotation-normalized horizontal regrouping for strongly vertical blocks.
- Line-local reversed token order.
- Numeric-aware stitching that rejoins amounts and decimal fragments.

The first implementation should keep the candidate set intentionally small.

## Safety Rules

- Only suspicious local blocks are eligible.
- Keep the original block unless the readability improvement crosses a margin threshold.
- Do not change page-level ordering logic.
- Keep explicit table recovery paths gated separately.

## Testing

- Add synthetic block tests for:
  - rotated single-character table-like text,
  - non-rotated reversed local text,
  - split numeric/currency fragments,
  - healthy blocks that must not be rewritten.
- Keep the existing `issue-140` render test green.
- Run the full suite to ensure no healthy table regressions.
