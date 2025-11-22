// Package mocks provides centralized mock implementations for testing.
//
// This package contains mock implementations of all repository interfaces
// and other dependencies used throughout the ViewRA application tests.
// All mocks follow consistent patterns for error injection and test helpers.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    mediaRepo := mocks.NewMediaRepository(t).
//	        WithMedia(testMedia1, testMedia2)
//
//	    // Use the mock in your test
//	    result, err := useCase.Execute(ctx, mediaRepo)
//	}
package mocks
