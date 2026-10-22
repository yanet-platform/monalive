// Package coalescer provides a utility function for selecting the first non-nil
// pointer from a list of pointers. This can be particularly useful in scenarios
// where you want to prioritize multiple potential values, taking the first
// available one.
package coalescer

// Coalesce returns the first non-nil pointer from the given list of pointers.
// If all pointers are nil, it returns nil.
//
// Example usage:
//
//	val1 := 1
//	val2 := 2
//	result := Coalesce(&val1, nil, &val2) // result will point to val1
func Coalesce[T any](values ...*T) *T {
	for _, v := range values {
		if v != nil {
			return v // Return the first non-nil pointer
		}
	}
	return nil // Return nil if all pointers are nil
}
