package bus

import (
	"sync"
	"testing"
)

func TestNewRingBuffer(t *testing.T) {
	rb := NewRingBuffer[int](100)

	if rb.Capacity() != 100 {
		t.Errorf("expected capacity 100, got %d", rb.Capacity())
	}
	if rb.Count() != 0 {
		t.Errorf("expected count 0, got %d", rb.Count())
	}
	if rb.Sequence() != 0 {
		t.Errorf("expected sequence 0, got %d", rb.Sequence())
	}
}

func TestNewRingBuffer_ZeroCapacity(t *testing.T) {
	rb := NewRingBuffer[int](0)

	// Should default to 1000
	if rb.Capacity() != 1000 {
		t.Errorf("expected default capacity 1000, got %d", rb.Capacity())
	}
}

func TestNewRingBuffer_NegativeCapacity(t *testing.T) {
	rb := NewRingBuffer[int](-5)

	// Should default to 1000
	if rb.Capacity() != 1000 {
		t.Errorf("expected default capacity 1000, got %d", rb.Capacity())
	}
}

func TestRingBuffer_Add(t *testing.T) {
	rb := NewRingBuffer[int](10)

	rb.Add(1)
	rb.Add(2)
	rb.Add(3)

	if rb.Count() != 3 {
		t.Errorf("expected count 3, got %d", rb.Count())
	}
	if rb.Sequence() != 3 {
		t.Errorf("expected sequence 3, got %d", rb.Sequence())
	}
}

func TestRingBuffer_Add_Overflow(t *testing.T) {
	rb := NewRingBuffer[int](3)

	// Add more items than capacity
	for i := 1; i <= 5; i++ {
		rb.Add(i)
	}

	// Count should be capped at capacity
	if rb.Count() != 3 {
		t.Errorf("expected count 3, got %d", rb.Count())
	}
	// Sequence tracks total adds
	if rb.Sequence() != 5 {
		t.Errorf("expected sequence 5, got %d", rb.Sequence())
	}

	// Should have last 3 items: 3, 4, 5
	items := rb.Last(3)
	expected := []int{3, 4, 5}
	for i, v := range items {
		if v != expected[i] {
			t.Errorf("position %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestRingBuffer_Last(t *testing.T) {
	rb := NewRingBuffer[int](10)

	for i := 1; i <= 5; i++ {
		rb.Add(i)
	}

	// Get last 3
	items := rb.Last(3)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	expected := []int{3, 4, 5}
	for i, v := range items {
		if v != expected[i] {
			t.Errorf("position %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestRingBuffer_Last_MoreThanAvailable(t *testing.T) {
	rb := NewRingBuffer[int](10)

	rb.Add(1)
	rb.Add(2)

	// Request more than available
	items := rb.Last(5)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	expected := []int{1, 2}
	for i, v := range items {
		if v != expected[i] {
			t.Errorf("position %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestRingBuffer_Last_Zero(t *testing.T) {
	rb := NewRingBuffer[int](10)

	rb.Add(1)
	rb.Add(2)

	items := rb.Last(0)
	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}

func TestRingBuffer_Last_Negative(t *testing.T) {
	rb := NewRingBuffer[int](10)

	rb.Add(1)
	rb.Add(2)

	items := rb.Last(-5)
	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}

func TestRingBuffer_Last_Empty(t *testing.T) {
	rb := NewRingBuffer[int](10)

	items := rb.Last(5)
	if len(items) != 0 {
		t.Errorf("expected empty slice, got %v", items)
	}
}

func TestRingBuffer_Last_All(t *testing.T) {
	rb := NewRingBuffer[int](5)

	for i := 1; i <= 5; i++ {
		rb.Add(i)
	}

	items := rb.Last(5)
	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(items))
	}
	for i, v := range items {
		if v != i+1 {
			t.Errorf("position %d: expected %d, got %d", i, i+1, v)
		}
	}
}

func TestRingBuffer_Last_AfterOverflow(t *testing.T) {
	rb := NewRingBuffer[int](3)

	// Add 7 items to a buffer of size 3
	for i := 1; i <= 7; i++ {
		rb.Add(i)
	}

	// Should have 5, 6, 7
	items := rb.Last(3)
	expected := []int{5, 6, 7}
	for i, v := range items {
		if v != expected[i] {
			t.Errorf("position %d: expected %d, got %d", i, expected[i], v)
		}
	}

	// Get only last 2
	items = rb.Last(2)
	expected = []int{6, 7}
	for i, v := range items {
		if v != expected[i] {
			t.Errorf("position %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestRingBuffer_Clear(t *testing.T) {
	rb := NewRingBuffer[int](10)

	for i := 1; i <= 5; i++ {
		rb.Add(i)
	}

	rb.Clear()

	if rb.Count() != 0 {
		t.Errorf("expected count 0 after clear, got %d", rb.Count())
	}
	// Sequence is not reset
	// Note: Based on the implementation, Clear() doesn't reset sequence
	// If that's the intended behavior, this test documents it

	items := rb.Last(5)
	if len(items) != 0 {
		t.Errorf("expected empty slice after clear, got %v", items)
	}
}

func TestRingBuffer_Concurrency(t *testing.T) {
	rb := NewRingBuffer[int](1000)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rb.Add(offset*100 + j)
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = rb.Last(10)
				_ = rb.Count()
				_ = rb.Sequence()
			}
		}()
	}

	wg.Wait()

	// After all operations, we should have written 1000 items
	if rb.Sequence() != 1000 {
		t.Errorf("expected sequence 1000, got %d", rb.Sequence())
	}
	if rb.Count() != 1000 {
		t.Errorf("expected count 1000, got %d", rb.Count())
	}
}

func TestRingBuffer_Generic_String(t *testing.T) {
	rb := NewRingBuffer[string](5)

	rb.Add("a")
	rb.Add("b")
	rb.Add("c")

	items := rb.Last(3)
	expected := []string{"a", "b", "c"}
	for i, v := range items {
		if v != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], v)
		}
	}
}

func TestRingBuffer_Generic_Struct(t *testing.T) {
	type Item struct {
		ID   int
		Name string
	}

	rb := NewRingBuffer[Item](3)

	rb.Add(Item{1, "one"})
	rb.Add(Item{2, "two"})
	rb.Add(Item{3, "three"})
	rb.Add(Item{4, "four"}) // Overwrites first

	items := rb.Last(3)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].ID != 2 || items[0].Name != "two" {
		t.Errorf("expected {2, two}, got %v", items[0])
	}
	if items[2].ID != 4 || items[2].Name != "four" {
		t.Errorf("expected {4, four}, got %v", items[2])
	}
}
