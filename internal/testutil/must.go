package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Must takes return values from a function and returns the non-error one. If
// the error value is non-nil then it fails the test
func Must[T any](val T, err error) func(*testing.T) T {
	return func(t *testing.T) T {
		require.NoError(t, err)
		return val
	}
}
