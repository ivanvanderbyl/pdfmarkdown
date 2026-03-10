package pdfmarkdown

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

func recoverSuspiciousTextBlocks(blocks []TextBlock) []TextBlock {
	if len(blocks) == 0 {
		return blocks
	}

	recovered := make([]TextBlock, len(blocks))
	for i, block := range blocks {
		if normalized, changed := recoverSuspiciousTextBlock(block); changed {
			recovered[i] = normalized
			continue
		}
		recovered[i] = block
	}

	return recovered
}

func normalizeIssue140RotatedBlocks(blocks []TextBlock) []TextBlock {
	return recoverSuspiciousTextBlocks(blocks)
}

func recoverSuspiciousTextBlock(block TextBlock) (TextBlock, bool) {
	if len(block.Words) == 0 {
		return block, false
	}

	if len(block.Lines) == 0 {
		block.Lines = groupWordsIntoLinesWithRotation(block.Words, block.Rotation)
	}

	if isLikelyIssue140RotatedBlock(block) {
		return normalizeIssue140RotatedBlock(block), true
	}

	encodingRecovered, encodingChanged := recoverSuspiciousEncodingBlock(block)
	if encodingChanged {
		block = encodingRecovered
	}

	garbleScore := scoreBlockGarble(block)
	if garbleScore < 1.0 {
		return block, encodingChanged
	}

	original := normalizeBlockCandidate(block)
	best := original
	bestScore := scoreBlockReadability(original)

	candidates := []TextBlock{
		original,
	}

	if isVerticalBlock(block) {
		candidates = append(candidates, normalizeRotatedSuspiciousBlock(block))
	}

	for _, candidate := range candidates {
		score := scoreBlockReadability(candidate)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}

	if blockText(best) == blockText(block) {
		return block, encodingChanged
	}

	return best, true
}

func shouldNormalizeIssue140RotatedBlock(block TextBlock) bool {
	return isLikelyIssue140RotatedBlock(block)
}

func isLikelyIssue140RotatedBlock(block TextBlock) bool {
	angle := normalizeAngle(block.Rotation)
	if !((angle >= 225 && angle < 315) || (angle >= 45 && angle < 135)) {
		return false
	}
	if len(block.Words) < 150 || len(block.Lines) < 8 {
		return false
	}

	singleChar := 0
	longLines := 0
	for _, word := range block.Words {
		if len([]rune(word.Text)) == 1 {
			singleChar++
		}
	}
	for _, line := range block.Lines {
		if len(line.Words) >= 40 {
			longLines++
		}
	}

	return float64(singleChar)/float64(len(block.Words)) >= 0.85 && longLines >= 3
}

func isVerticalBlock(block TextBlock) bool {
	angle := normalizeAngle(block.Rotation)
	return (angle >= 45 && angle < 135) || (angle >= 225 && angle < 315)
}

func scoreBlockGarble(block TextBlock) float64 {
	score := 0.0

	singleCharRatio := blockSingleCharacterRatio(block)
	if singleCharRatio >= 0.8 {
		score += 2.5
	} else if singleCharRatio >= 0.45 {
		score += 1.2
	}

	fragmentedNumericGroups := countFragmentedNumericGroups(block)
	score += float64(fragmentedNumericGroups) * 1.0

	fragmentedAlphaGroups := countFragmentedAlphabeticGroups(block)
	score += float64(fragmentedAlphaGroups) * 0.8

	mirroredTokens := countMirroredAlphaTokens(block)
	if mirroredTokens >= 2 {
		score += 1.6
	}

	if isLikelyIssue140RotatedBlock(block) {
		score += 2.0
	}

	return score
}

func scoreBlockReadability(block TextBlock) float64 {
	score := 0.0
	for _, line := range block.Lines {
		score += scoreLineReadability(line)
	}
	return score
}

func scoreLineReadability(line Line) float64 {
	score := 0.0
	for _, word := range line.Words {
		score += scoreTokenReadability(word.Text)
	}

	score -= float64(countFragmentedNumericWordGroups(line.Words)) * 0.75
	score -= float64(countFragmentedAlphabeticWordGroups(line.Words)) * 0.5
	score -= float64(countSingleCharAlphaWords(line.Words)) * 0.35
	return score
}

func scoreTokenReadability(token string) float64 {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0
	}

	if looksCurrencyOrAmountToken(token) {
		return 2.2
	}
	if isNumericToken(token) {
		return 1.0
	}

	runes := []rune(token)
	alphaCount := 0
	vowelCount := 0
	longestConsonantRun := 0
	currentConsonantRun := 0

	for _, r := range runes {
		if !unicode.IsLetter(r) {
			continue
		}
		alphaCount++
		if isVowelRune(unicode.ToLower(r)) {
			vowelCount++
			currentConsonantRun = 0
		} else {
			currentConsonantRun++
			if currentConsonantRun > longestConsonantRun {
				longestConsonantRun = currentConsonantRun
			}
		}
	}

	if alphaCount == 0 {
		return 0.2
	}
	if alphaCount == 1 {
		return -0.7
	}

	score := 0.6
	if vowelCount > 0 {
		score += 0.6
	}
	if longestConsonantRun >= 5 {
		score -= 0.8
	}
	if alphaCount >= 6 && vowelCount == 0 {
		score -= 0.8
	}

	reversed := reverseString(token)
	if reversed != token {
		reversedScore := scoreTokenReadabilityBase(reversed)
		currentScore := scoreTokenReadabilityBase(token)
		if reversedScore > currentScore+0.8 {
			score -= 0.8
		}
	}

	return score
}

func scoreTokenReadabilityBase(token string) float64 {
	runes := []rune(token)
	alphaCount := 0
	vowelCount := 0
	longestConsonantRun := 0
	currentConsonantRun := 0
	for _, r := range runes {
		if !unicode.IsLetter(r) {
			continue
		}
		alphaCount++
		if isVowelRune(unicode.ToLower(r)) {
			vowelCount++
			currentConsonantRun = 0
		} else {
			currentConsonantRun++
			if currentConsonantRun > longestConsonantRun {
				longestConsonantRun = currentConsonantRun
			}
		}
	}

	score := float64(vowelCount) * 0.25
	score -= float64(longestConsonantRun) * 0.1
	if alphaCount >= 4 && vowelCount == 0 {
		score -= 0.5
	}
	return score
}

func normalizeBlockCandidate(block TextBlock) TextBlock {
	lines := make([]Line, 0, len(block.Lines))
	for _, line := range block.Lines {
		words := stitchSingleCharacterWords(line.Words)
		words = stitchFragmentedAlphabeticWords(words)
		words = stitchFragmentedNumericWords(words)
		lines = append(lines, rebuildLine(words))
	}

	return rebuildBlock(lines, 0, "ltr")
}

func normalizeRotatedSuspiciousBlock(block TextBlock) TextBlock {
	angle := normalizeAngle(block.Rotation)
	rotateBy := 360 - angle
	if rotateBy > 180 {
		rotateBy -= 360
	}

	rotatedWords := make([]EnrichedWord, len(block.Words))
	for i, word := range block.Words {
		rotatedWords[i] = rotateWordToHorizontal(word, rotateBy)
	}

	lines := groupWordsIntoHorizontalLines(rotatedWords)
	rotated := TextBlock{
		Words:            rotatedWords,
		Lines:            lines,
		Rotation:         0,
		ReadingDirection: "ltr",
	}

	return normalizeBlockCandidate(rotated)
}

func normalizeIssue140RotatedBlock(block TextBlock) TextBlock {
	angle := normalizeAngle(block.Rotation)
	rotateBy := 360 - angle
	if rotateBy > 180 {
		rotateBy -= 360
	}

	rotatedWords := make([]EnrichedWord, len(block.Words))
	for i, word := range block.Words {
		rotatedWords[i] = rotateWordToHorizontal(word, rotateBy)
	}

	lines := groupWordsIntoHorizontalLines(rotatedWords)
	for i := range lines {
		lines[i].Words = stitchRotatedSingleCharacterWords(lines[i].Words)
		if len(lines[i].Words) == 0 {
			continue
		}
		lines[i].Box = lines[i].Words[0].Box
		for _, word := range lines[i].Words[1:] {
			lines[i].Box = mergeRects(lines[i].Box, word.Box)
		}
		lines[i].Baseline = lines[i].Words[0].Baseline
	}

	var normalizedWords []EnrichedWord
	for _, line := range lines {
		normalizedWords = append(normalizedWords, line.Words...)
	}

	return TextBlock{
		Words:            normalizedWords,
		Lines:            lines,
		Rotation:         0,
		ReadingDirection: "ltr",
	}
}

func reverseSuspiciousTokensBlock(block TextBlock) TextBlock {
	lines := make([]Line, 0, len(block.Lines))
	for _, line := range block.Lines {
		words := cloneWords(line.Words)
		for i := range words {
			if shouldReverseToken(words[i].Text) {
				words[i].Text = reverseString(words[i].Text)
			}
		}
		lines = append(lines, rebuildLine(words))
	}
	return rebuildBlock(lines, block.Rotation, block.ReadingDirection)
}

func reverseSuspiciousTokensAndOrderBlock(block TextBlock) TextBlock {
	lines := make([]Line, 0, len(block.Lines))
	for _, line := range block.Lines {
		words := cloneWords(line.Words)
		for i := range words {
			if shouldReverseToken(words[i].Text) {
				words[i].Text = reverseString(words[i].Text)
			}
		}
		for i := 0; i < len(words)/2; i++ {
			j := len(words) - 1 - i
			words[i], words[j] = words[j], words[i]
		}
		lines = append(lines, rebuildLine(words))
	}
	return rebuildBlock(lines, block.Rotation, block.ReadingDirection)
}

func shouldReverseToken(token string) bool {
	if len([]rune(token)) < 4 || !isAlphabeticToken(token) {
		return false
	}
	reversed := reverseString(token)
	return scoreTokenReadabilityBase(reversed) > scoreTokenReadabilityBase(token)+0.8
}

func stitchSingleCharacterWords(words []EnrichedWord) []EnrichedWord {
	if len(words) <= 1 {
		return words
	}

	singleChars := countSingleCharAlphaWords(words)
	if float64(singleChars)/float64(len(words)) < 0.6 {
		return words
	}

	gaps := make([]float64, 0, len(words)-1)
	avgWidth := 0.0
	for i, word := range words {
		avgWidth += word.Box.Width()
		if i == 0 {
			continue
		}
		gap := word.Box.X0 - words[i-1].Box.X1
		if gap >= 0 {
			gaps = append(gaps, gap)
		}
	}
	avgWidth /= float64(len(words))

	joinThreshold := avgWidth * 0.9
	if len(gaps) > 0 {
		sortedGaps := append([]float64(nil), gaps...)
		sort.Float64s(sortedGaps)
		sampleEnd := int(math.Ceil(float64(len(sortedGaps)) * 0.6))
		if sampleEnd < 1 {
			sampleEnd = 1
		}
		smallMedian := sortedGaps[sampleEnd/2]
		joinThreshold = math.Max(avgWidth*0.35, math.Min(avgWidth*1.2, smallMedian*1.8))
	}

	var stitched []EnrichedWord
	current := []EnrichedWord{words[0]}
	for i := 1; i < len(words); i++ {
		prev := words[i-1]
		curr := words[i]
		gap := curr.Box.X0 - prev.Box.X1
		if gap <= joinThreshold {
			current = append(current, curr)
			continue
		}

		stitched = append(stitched, mergeWordGroup(current))
		current = []EnrichedWord{curr}
	}
	if len(current) > 0 {
		stitched = append(stitched, mergeWordGroup(current))
	}

	return stitched
}

func stitchRotatedSingleCharacterWords(words []EnrichedWord) []EnrichedWord {
	if len(words) <= 1 {
		return words
	}

	singleChars := 0
	for _, word := range words {
		if len([]rune(word.Text)) == 1 {
			singleChars++
		}
	}
	if float64(singleChars)/float64(len(words)) < 0.8 {
		return words
	}

	gaps := make([]float64, 0, len(words)-1)
	avgWidth := 0.0
	for i, word := range words {
		avgWidth += word.Box.Width()
		if i == 0 {
			continue
		}
		gap := word.Box.X0 - words[i-1].Box.X1
		if gap >= 0 {
			gaps = append(gaps, gap)
		}
	}
	avgWidth /= float64(len(words))

	joinThreshold := avgWidth * 0.9
	if len(gaps) > 0 {
		sortedGaps := append([]float64(nil), gaps...)
		sort.Float64s(sortedGaps)
		sampleEnd := int(math.Ceil(float64(len(sortedGaps)) * 0.6))
		if sampleEnd < 1 {
			sampleEnd = 1
		}
		smallMedian := sortedGaps[sampleEnd/2]
		joinThreshold = math.Max(avgWidth*0.35, math.Min(avgWidth*1.2, smallMedian*1.8))
	}

	var stitched []EnrichedWord
	current := []EnrichedWord{words[0]}
	for i := 1; i < len(words); i++ {
		prev := words[i-1]
		curr := words[i]
		gap := curr.Box.X0 - prev.Box.X1
		if gap <= joinThreshold {
			current = append(current, curr)
			continue
		}

		stitched = append(stitched, mergeWordGroup(current))
		current = []EnrichedWord{curr}
	}
	if len(current) > 0 {
		stitched = append(stitched, mergeWordGroup(current))
	}

	return stitched
}

func stitchFragmentedNumericWords(words []EnrichedWord) []EnrichedWord {
	if len(words) <= 1 {
		return words
	}

	stitched := make([]EnrichedWord, 0, len(words))
	for i := 0; i < len(words); {
		if !isNumericFragmentWord(words[i].Text) {
			stitched = append(stitched, words[i])
			i++
			continue
		}

		bestEnd := -1
		bestText := ""
		for j := i + 1; j <= len(words) && isNumericFragmentWord(words[j-1].Text); j++ {
			candidate := joinWordTexts(words[i:j])
			if looksCurrencyOrAmountToken(candidate) || isNumericToken(candidate) {
				bestEnd = j
				bestText = candidate
			}
			if j == len(words) || !isNumericFragmentWord(words[j].Text) {
				break
			}
		}

		if bestEnd > i+1 {
			stitched = append(stitched, mergeWordGroupWithText(words[i:bestEnd], bestText))
			i = bestEnd
			continue
		}

		stitched = append(stitched, words[i])
		i++
	}

	return stitched
}

func stitchFragmentedAlphabeticWords(words []EnrichedWord) []EnrichedWord {
	if len(words) <= 1 {
		return words
	}

	stitched := make([]EnrichedWord, 0, len(words))
	current := words[0]
	for i := 1; i < len(words); i++ {
		if shouldMergeAlphabeticFragments(current, words[i]) {
			current = mergeWordGroupWithText([]EnrichedWord{current, words[i]}, current.Text+words[i].Text)
			continue
		}
		stitched = append(stitched, current)
		current = words[i]
	}
	stitched = append(stitched, current)
	return stitched
}

func blockSingleCharacterRatio(block TextBlock) float64 {
	if len(block.Words) == 0 {
		return 0
	}

	single := 0
	for _, word := range block.Words {
		if len([]rune(strings.TrimSpace(word.Text))) == 1 {
			single++
		}
	}
	return float64(single) / float64(len(block.Words))
}

func countFragmentedNumericGroups(block TextBlock) int {
	count := 0
	for _, line := range block.Lines {
		count += countFragmentedNumericWordGroups(line.Words)
	}
	return count
}

func countFragmentedAlphabeticGroups(block TextBlock) int {
	count := 0
	for _, line := range block.Lines {
		count += countFragmentedAlphabeticWordGroups(line.Words)
	}
	return count
}

func countFragmentedAlphabeticWordGroups(words []EnrichedWord) int {
	count := 0
	for i := 1; i < len(words); i++ {
		if shouldMergeAlphabeticFragments(words[i-1], words[i]) {
			count++
		}
	}
	return count
}

func countFragmentedNumericWordGroups(words []EnrichedWord) int {
	count := 0
	run := 0
	for _, word := range words {
		if isNumericFragmentWord(word.Text) {
			run++
			continue
		}
		if run >= 2 {
			count++
		}
		run = 0
	}
	if run >= 2 {
		count++
	}
	return count
}

func countMirroredAlphaTokens(block TextBlock) int {
	count := 0
	for _, word := range block.Words {
		if shouldReverseToken(word.Text) {
			count++
		}
	}
	return count
}

func countSingleCharAlphaWords(words []EnrichedWord) int {
	count := 0
	for _, word := range words {
		runes := []rune(strings.TrimSpace(word.Text))
		if len(runes) == 1 && unicode.IsLetter(runes[0]) {
			count++
		}
	}
	return count
}

func shouldMergeAlphabeticFragments(left, right EnrichedWord) bool {
	if !isAlphabeticToken(left.Text) || !isAlphabeticToken(right.Text) {
		return false
	}

	gap := right.Box.X0 - left.Box.X1
	if gap < 0 {
		return false
	}

	avgFontSize := (left.FontSize + right.FontSize) / 2
	gapThreshold := avgFontSize * 0.25
	if len([]rune(left.Text)) <= 2 || len([]rune(right.Text)) <= 2 {
		gapThreshold = avgFontSize * 0.45
	}
	if gap > gapThreshold {
		return false
	}

	overlapY0 := math.Max(left.Box.Y0, right.Box.Y0)
	overlapY1 := math.Min(left.Box.Y1, right.Box.Y1)
	overlapHeight := overlapY1 - overlapY0
	minHeight := math.Min(left.Box.Height(), right.Box.Height())
	if minHeight <= 0 || overlapHeight/minHeight < 0.4 {
		return false
	}

	return len([]rune(left.Text))+len([]rune(right.Text)) >= 6
}

func isNumericFragmentWord(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if text == "$" {
		return true
	}
	if strings.HasPrefix(text, "$") {
		text = strings.TrimPrefix(text, "$")
		if text == "" {
			return true
		}
	}

	hasDigit := false
	for _, r := range text {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '.' || r == ',' || r == '-':
		default:
			return false
		}
	}

	return hasDigit
}

func looksCurrencyOrAmountToken(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if strings.HasPrefix(text, "$") {
		text = strings.TrimPrefix(text, "$")
	}
	if !strings.Contains(text, ".") {
		return false
	}
	return isNumericToken(text)
}

func isNumericToken(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if strings.HasPrefix(text, "$") {
		text = strings.TrimPrefix(text, "$")
	}
	dots := 0
	for _, r := range text {
		if r == '.' {
			dots++
		}
		if (r < '0' || r > '9') && r != '.' && r != ',' && r != '-' {
			return false
		}
	}
	return dots <= 1
}

func isVowelRune(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	default:
		return false
	}
}

func reverseString(text string) string {
	runes := []rune(text)
	for i := 0; i < len(runes)/2; i++ {
		j := len(runes) - 1 - i
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func joinWordTexts(words []EnrichedWord) string {
	var builder strings.Builder
	for _, word := range words {
		builder.WriteString(word.Text)
	}
	return builder.String()
}

func mergeWordGroupWithText(words []EnrichedWord, text string) EnrichedWord {
	merged := mergeWordGroup(words)
	merged.Text = text
	return merged
}

func rebuildLine(words []EnrichedWord) Line {
	if len(words) == 0 {
		return Line{}
	}

	line := Line{
		Words:    words,
		Box:      words[0].Box,
		Baseline: words[0].Baseline,
	}
	for _, word := range words[1:] {
		line.Box = mergeRects(line.Box, word.Box)
	}
	return line
}

func rebuildBlock(lines []Line, rotation float64, readingDirection string) TextBlock {
	var words []EnrichedWord
	for _, line := range lines {
		words = append(words, line.Words...)
	}

	return TextBlock{
		Words:            words,
		Lines:            lines,
		Rotation:         rotation,
		ReadingDirection: readingDirection,
	}
}

func blockText(block TextBlock) string {
	lines := make([]string, 0, len(block.Lines))
	for _, line := range block.Lines {
		lines = append(lines, lineText(line))
	}
	return strings.Join(lines, "\n")
}

func recoverSuspiciousEncodingBlock(block TextBlock) (TextBlock, bool) {
	if len(block.Lines) == 0 {
		return block, false
	}

	suspiciousTokens := 0
	for _, word := range block.Words {
		if isSuspiciousEncodingToken(word.Text) {
			suspiciousTokens++
		}
	}
	if suspiciousTokens < 2 || scoreBlockEncodingCorruption(block) < 2.0 {
		return block, false
	}

	lines := make([]Line, 0, len(block.Lines))
	changed := false
	for _, line := range block.Lines {
		kept := make([]EnrichedWord, 0, len(line.Words))
		for _, word := range line.Words {
			if shouldPreserveEncodingToken(word.Text) {
				kept = append(kept, word)
				continue
			}
			if isSuspiciousEncodingToken(word.Text) {
				changed = true
			} else {
				kept = append(kept, word)
			}
		}
		if len(kept) == 0 {
			changed = true
			continue
		}
		lines = append(lines, rebuildLine(kept))
		if len(kept) != len(line.Words) {
			changed = true
		}
	}

	if !changed {
		return block, false
	}
	if len(lines) == 0 {
		return block, false
	}

	return rebuildBlock(lines, block.Rotation, block.ReadingDirection), true
}

func scoreBlockEncodingCorruption(block TextBlock) float64 {
	score := 0.0
	suspicious := 0
	preserved := 0
	for _, word := range block.Words {
		if isSuspiciousEncodingToken(word.Text) {
			suspicious++
			score += 1.0
		}
		if shouldPreserveEncodingToken(word.Text) {
			preserved++
		}
	}

	if suspicious >= 3 {
		score += 0.8
	}
	if len(block.Words) > 0 && float64(suspicious)/float64(len(block.Words)) >= 0.5 {
		score += 0.6
	}
	if preserved == 0 {
		score += 0.4
	}
	return score
}

func shouldPreserveEncodingToken(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if isStructuredAnchorToken(text) {
		return true
	}
	if isReadableASCIIWord(text) {
		return true
	}
	return false
}

func isStructuredAnchorToken(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}

	hasDigit := false
	alpha := 0
	for _, r := range text {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			alpha++
		case r == '-' || r == '/' || r == ':' || r == '.' || r == ',' || r == '+' || r == '%':
		default:
			return false
		}
	}

	return hasDigit && (strings.ContainsAny(text, "-/:") || alpha >= 2)
}

func isReadableASCIIWord(text string) bool {
	text = strings.Trim(text, ".,:;()[]{}")
	if len(text) < 3 || !isAlphabeticToken(text) {
		return false
	}
	if isSuspiciousEncodingToken(text) {
		return false
	}
	return scoreTokenReadability(text) >= 1.0
}

func isSuspiciousEncodingToken(text string) bool {
	text = strings.TrimSpace(strings.Trim(text, ".,:;()[]{}"))
	if len(text) < 4 || strings.ContainsAny(text, "0123456789") {
		return false
	}
	if !isAlphabeticToken(text) {
		return false
	}

	unique := map[rune]struct{}{}
	dominant := 0
	alpha := 0
	for _, r := range text {
		lower := unicode.ToLower(r)
		if !unicode.IsLetter(lower) {
			continue
		}
		alpha++
		unique[lower] = struct{}{}
		if lower == 'a' || lower == 'b' {
			dominant++
		}
	}

	if alpha == 0 {
		return false
	}

	return len(unique) <= 3 && float64(dominant)/float64(alpha) >= 0.7
}

func cloneWords(words []EnrichedWord) []EnrichedWord {
	cloned := make([]EnrichedWord, len(words))
	copy(cloned, words)
	return cloned
}

func rotateWordToHorizontal(word EnrichedWord, angle float64) EnrichedWord {
	rotated := word
	rotated.Box = rotateRect(word.Box, angle)
	rotated.Rotation = 0
	rotated.Baseline = calculateBaseline(rotated)
	rotated.XHeight = calculateXHeight(rotated)
	return rotated
}
