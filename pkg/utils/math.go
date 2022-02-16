package utils

func IntMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// IntMax returns the maximum integer provided
func IntMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
