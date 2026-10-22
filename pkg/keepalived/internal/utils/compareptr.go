package utils

// ComparePtr is a generic function that compares two pointers of the same type.
func ComparePtr[T comparable](a, b *T) (equals bool) {
	return a == b || (a != nil && b != nil && *a == *b)
}
