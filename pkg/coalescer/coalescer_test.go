package coalescer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCoalescer tests the Coalesce function with a mix of nil and non-nil
// pointers. It expects the first non-nil pointer to be returned.
func TestCoalescer(t *testing.T) {
	num1 := 1
	num2 := 2
	num3 := 3

	// The function is called with two nil pointers followed by three non-nil
	// pointers. The expected result is a pointer to num1.
	result := Coalesce(nil, nil, &num1, &num2, &num3)
	assert.Equal(t, &num1, result)
}

// TestCoalescer_Empty tests the Coalesce function with an empty slice of
// pointers. It expects a nil result since there are no elements to check.
func TestCoalescer_Empty(t *testing.T) {
	numsPtr := []*int{}

	// The function is called with an empty slice of pointers. The expected
	// result is nil because there are no pointers in the slice.
	result := Coalesce(numsPtr...)
	assert.Nil(t, result)
}

// TestCoalescer_Nils tests the Coalesce function with a slice containing only
// nil pointers. It expects a nil result since there are no non-nil pointers.
func TestCoalescer_Nils(t *testing.T) {
	numsPtr := []*int{nil, nil, nil}

	// The function is called with a slice of nil pointers. The expected result
	// is nil because all pointers in the slice are nil.
	result := Coalesce(numsPtr...)
	assert.Nil(t, result)
}
