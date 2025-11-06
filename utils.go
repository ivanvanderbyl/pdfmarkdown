package pdfmarkdown

import (
	"math"
	"sort"
)

// calculateMedian calculates the median value of a float64 slice
func calculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

// calculateStdDev calculates the standard deviation of a float64 slice
func calculateStdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	mean := average(values)
	var sumSquares float64
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(values)))
}

// calculateBaseline estimates the baseline Y-coordinate for a word
// The baseline is typically at the bottom of non-descender characters
func calculateBaseline(word EnrichedWord) float64 {
	// For most fonts, baseline is approximately at Y1 (bottom of bounding box)
	// Adjust by a small factor for descenders
	return word.Box.Y1 - (word.FontSize * 0.15)
}

// calculateXHeight estimates the x-height (height of lowercase letters) for a word
// X-height is typically about 0.5-0.7 times the font size
func calculateXHeight(word EnrichedWord) float64 {
	// Check if word contains lowercase letters
	hasLowercase := false
	for _, r := range word.Text {
		if r >= 'a' && r <= 'z' {
			hasLowercase = true
			break
		}
	}

	if hasLowercase {
		// Use actual height for words with lowercase
		return word.Box.Height() * 0.7
	}

	// Estimate based on font size
	return word.FontSize * 0.5
}

// quantizeAngle rounds an angle to the nearest multiple of step degrees
func quantizeAngle(angle, step float64) float64 {
	return math.Round(angle/step) * step
}

// normalizeAngle normalizes an angle to [0, 360) range
func normalizeAngle(angle float64) float64 {
	angle = math.Mod(angle, 360)
	if angle < 0 {
		angle += 360
	}
	return angle
}

// inferReadingDirection infers reading direction from rotation angle
func inferReadingDirection(angle float64) string {
	angle = normalizeAngle(angle)

	switch {
	case angle < 45 || angle >= 315:
		return "ltr" // left-to-right (horizontal)
	case angle >= 45 && angle < 135:
		return "ttb" // top-to-bottom (vertical, rotated 90°)
	case angle >= 135 && angle < 225:
		return "rtl" // right-to-left (horizontal, rotated 180°)
	default:
		return "btt" // bottom-to-top (vertical, rotated 270°)
	}
}

// angleBetween calculates the angle between two points
func angleBetween(x0, y0, x1, y1 float64) float64 {
	return math.Atan2(y1-y0, x1-x0) * 180 / math.Pi
}

// rotatePoint rotates a point around the origin by angle degrees
func rotatePoint(x, y, angle float64) (float64, float64) {
	rad := angle * math.Pi / 180
	cos := math.Cos(rad)
	sin := math.Sin(rad)

	newX := x*cos - y*sin
	newY := x*sin + y*cos

	return newX, newY
}

// rotateRect rotates a rectangle around the origin by angle degrees
// Returns the axis-aligned bounding box of the rotated rectangle
func rotateRect(rect Rect, angle float64) Rect {
	// Rotate all four corners
	x0, y0 := rotatePoint(rect.X0, rect.Y0, angle)
	x1, y1 := rotatePoint(rect.X1, rect.Y0, angle)
	x2, y2 := rotatePoint(rect.X1, rect.Y1, angle)
	x3, y3 := rotatePoint(rect.X0, rect.Y1, angle)

	// Find bounding box
	minX := math.Min(math.Min(x0, x1), math.Min(x2, x3))
	maxX := math.Max(math.Max(x0, x1), math.Max(x2, x3))
	minY := math.Min(math.Min(y0, y1), math.Min(y2, y3))
	maxY := math.Max(math.Max(y0, y1), math.Max(y2, y3))

	return Rect{
		X0: minX,
		Y0: minY,
		X1: maxX,
		Y1: maxY,
	}
}

// rectsOverlap checks if two rectangles overlap
func rectsOverlap(r1, r2 Rect) bool {
	return !(r1.X1 <= r2.X0 || r2.X1 <= r1.X0 || r1.Y1 <= r2.Y0 || r2.Y1 <= r1.Y0)
}

// rectContains checks if rect1 contains rect2
func rectContains(r1, r2 Rect) bool {
	return r1.X0 <= r2.X0 && r1.Y0 <= r2.Y0 && r1.X1 >= r2.X1 && r1.Y1 >= r2.Y1
}

// expandRect expands a rectangle by the given amount in all directions
func expandRect(rect Rect, amount float64) Rect {
	return Rect{
		X0: rect.X0 - amount,
		Y0: rect.Y0 - amount,
		X1: rect.X1 + amount,
		Y1: rect.Y1 + amount,
	}
}

// mergeRects merges two rectangles into their bounding box
func mergeRects(r1, r2 Rect) Rect {
	return Rect{
		X0: math.Min(r1.X0, r2.X0),
		Y0: math.Min(r1.Y0, r2.Y0),
		X1: math.Max(r1.X1, r2.X1),
		Y1: math.Max(r1.Y1, r2.Y1),
	}
}

// clamp restricts a value to a range
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
