package ui

func clampDimension(value int) int {
	if value < 1 {
		return 1
	}
	return value
}
