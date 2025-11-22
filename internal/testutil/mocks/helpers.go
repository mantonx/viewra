package mocks

import "testing"

// NewTestID generates a test ID from a base value.
// This is useful for creating predictable test data.
func NewTestID(base int64) int64 {
	return base
}

// RequireNoError is a test helper that fails the test if err is not nil.
// It uses t.Helper() to ensure the failure is reported at the correct location.
func RequireNoError(t testing.TB, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// AssertEqual is a simple equality assertion helper.
func AssertEqual(t testing.TB, expected, actual interface{}, msg string) {
	t.Helper()
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", msg, expected, actual)
	}
}

// AssertNotNil fails the test if value is nil.
func AssertNotNil(t testing.TB, value interface{}, msg string) {
	t.Helper()
	if value == nil {
		t.Errorf("%s: expected non-nil value", msg)
	}
}
