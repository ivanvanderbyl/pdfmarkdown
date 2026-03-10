# Issue 192 Suspicious Encoding Recovery Design

## Goal

Improve `testdata/issue-192-example.pdf` by treating it as a high-confidence encoding-corruption problem at the local block level, prioritizing readable text and suppressing low-confidence gibberish.

## Constraints

- Recovery must stay local to suspicious blocks.
- Low-confidence character repair is not allowed.
- Corrupted runs should be suppressed rather than guessed.
- Healthy pages and healthy blocks must remain unchanged.
- Existing table detection should continue to operate, but not on obviously corrupted cell text.

## Failure Shape

`issue-192` is not just a reading-order problem. The current output shows:

- repeated `A/a/b`-style substitutions,
- very low character diversity inside long text runs,
- some useful structural anchors still present, such as numbers, punctuation, separators, and a few readable ASCII tokens,
- table extraction happening on top of already-corrupted tokens.

## Proposed Architecture

Add a suspicious-encoding recovery stage adjacent to the current suspicious-block recovery.

For each `TextBlock`:

1. Score encoding corruption from:
   - low alphabet diversity,
   - extreme dominance of a tiny repeated alphabet,
   - long mixed-case gibberish runs,
   - low readable-token density.
2. Skip blocks below the threshold.
3. For suspicious blocks:
   - preserve high-confidence structured tokens,
   - suppress low-confidence gibberish tokens or runs,
   - apply only tiny, high-confidence repairs where the evidence is local and unambiguous.

The result should be a cleaner block with readable anchors preserved and corrupted noise removed.

## High-Confidence Preservation Rules

Keep tokens that strongly look like:

- dates,
- numbers, currency, percentages,
- account/ID/code-style alphanumerics,
- punctuation or separators that contribute structure,
- readable ASCII words that survive normal lexical checks.

Suppress tokens when they are dominated by the suspicious encoding alphabet and fail readability checks.

## Markdown / Table Impact

- Cleaner paragraphs by removing corrupted runs from the text flow.
- Tables may remain structurally present, but corrupted cells should become empty or sparse rather than misleading gibberish.
- This is acceptable because the user explicitly prefers readable high-confidence text over speculative reconstruction.

## Testing

- Add synthetic block tests for:
  - corrupted runs with embedded numeric anchors,
  - readable tokens mixed with low-confidence encoding noise,
  - healthy mixed-case text that must not be suppressed.
- Keep `issue-192` regression focused on:
  - markdown remains non-empty,
  - obvious corruption volume is reduced,
  - at least the known useful anchors remain,
  - table count is not catastrophically regressed.
