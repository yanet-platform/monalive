// Package throttler provides tool for determin whether repeated event should be
// throttled.
package throttler

// ThrottlerFunc is a function type that returns true if the value should be
// throttled.
type ThrottlerFunc func(value uint64) bool

// throttlerFunc is a global variable that holds the throttler function.
var throttlerFunc ThrottlerFunc = func(value uint64) bool { return !isPowerOfTwo(value) }

// SetThrottler sets the throttler function for the package.
func SetThrottler(throttler ThrottlerFunc) {
	throttlerFunc = throttler
}

// Throttle returns true if the value should be throttled.
//
// If the value is less than or equal to the limit, it returns false. Otherwise,
// it returns the result of the throttler function.
func Throttle(value, limit uint64) bool {
	if value <= limit {
		return false
	}
	return throttlerFunc(value)
}

// isPowerOfTwo returns true if the value is a power of two.
func isPowerOfTwo(value uint64) bool {
	return value != 0 && (value&(value-1)) == 0
}
