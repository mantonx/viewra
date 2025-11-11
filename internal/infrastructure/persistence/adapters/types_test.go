package adapters

import (
	"database/sql"
	"testing"
	"time"
)

func TestTypeAdapter_ConvertStruct_SameType(t *testing.T) {
	type TestStruct struct {
		ID    int64
		Name  string
		Count int
	}

	adapter := NewTypeAdapter()
	src := TestStruct{ID: 123, Name: "test", Count: 42}
	var dst TestStruct

	err := adapter.ConvertStruct(src, &dst)
	if err != nil {
		t.Fatalf("ConvertStruct failed: %v", err)
	}

	if dst.ID != src.ID || dst.Name != src.Name || dst.Count != src.Count {
		t.Errorf("Fields not copied correctly. Got %+v, want %+v", dst, src)
	}
}

func TestTypeAdapter_ConvertStruct_Int32ToInt64(t *testing.T) {
	type Source struct {
		ID int32
	}
	type Dest struct {
		ID int64
	}

	adapter := NewTypeAdapter()
	src := Source{ID: 42}
	var dst Dest

	err := adapter.ConvertStruct(src, &dst)
	if err != nil {
		t.Fatalf("ConvertStruct failed: %v", err)
	}

	if dst.ID != 42 {
		t.Errorf("ID = %d, want 42", dst.ID)
	}
}

func TestTypeAdapter_ConvertStruct_Int64ToInt32(t *testing.T) {
	type Source struct {
		ID int64
	}
	type Dest struct {
		ID int32
	}

	adapter := NewTypeAdapter()
	src := Source{ID: 12345}
	var dst Dest

	err := adapter.ConvertStruct(src, &dst)
	if err != nil {
		t.Fatalf("ConvertStruct failed: %v", err)
	}

	if dst.ID != 12345 {
		t.Errorf("ID = %d, want 12345", dst.ID)
	}
}

func TestTypeAdapter_ConvertStruct_NullInt32ToNullInt64(t *testing.T) {
	type Source struct {
		Count sql.NullInt32
	}
	type Dest struct {
		Count sql.NullInt64
	}

	adapter := NewTypeAdapter()

	// Test with valid value
	src := Source{Count: sql.NullInt32{Int32: 100, Valid: true}}
	var dst Dest

	err := adapter.ConvertStruct(src, &dst)
	if err != nil {
		t.Fatalf("ConvertStruct failed: %v", err)
	}

	if !dst.Count.Valid || dst.Count.Int64 != 100 {
		t.Errorf("Count = %+v, want {Int64: 100, Valid: true}", dst.Count)
	}

	// Test with NULL value
	src = Source{Count: sql.NullInt32{Valid: false}}
	dst = Dest{}

	err = adapter.ConvertStruct(src, &dst)
	if err != nil {
		t.Fatalf("ConvertStruct failed for NULL: %v", err)
	}

	if dst.Count.Valid {
		t.Errorf("Count.Valid = true, want false")
	}
}

func TestTypeAdapter_ConvertStruct_NullInt64ToNullInt32(t *testing.T) {
	type Source struct {
		Count sql.NullInt64
	}
	type Dest struct {
		Count sql.NullInt32
	}

	adapter := NewTypeAdapter()

	// Test with valid value
	src := Source{Count: sql.NullInt64{Int64: 200, Valid: true}}
	var dst Dest

	err := adapter.ConvertStruct(src, &dst)
	if err != nil {
		t.Fatalf("ConvertStruct failed: %v", err)
	}

	if !dst.Count.Valid || dst.Count.Int32 != 200 {
		t.Errorf("Count = %+v, want {Int32: 200, Valid: true}", dst.Count)
	}
}

func TestTypeAdapter_ConvertStruct_MixedTypes(t *testing.T) {
	type Source struct {
		ID        int32
		Name      string
		Count     sql.NullInt32
		Price     sql.NullFloat64
		Active    bool
		CreatedAt time.Time
	}
	type Dest struct {
		ID        int64
		Name      string
		Count     sql.NullInt64
		Price     sql.NullFloat64
		Active    bool
		CreatedAt time.Time
	}

	adapter := NewTypeAdapter()
	now := time.Now()
	src := Source{
		ID:        123,
		Name:      "test item",
		Count:     sql.NullInt32{Int32: 42, Valid: true},
		Price:     sql.NullFloat64{Float64: 19.99, Valid: true},
		Active:    true,
		CreatedAt: now,
	}
	var dst Dest

	err := adapter.ConvertStruct(src, &dst)
	if err != nil {
		t.Fatalf("ConvertStruct failed: %v", err)
	}

	if dst.ID != 123 {
		t.Errorf("ID = %d, want 123", dst.ID)
	}
	if dst.Name != "test item" {
		t.Errorf("Name = %q, want %q", dst.Name, "test item")
	}
	if !dst.Count.Valid || dst.Count.Int64 != 42 {
		t.Errorf("Count = %+v, want {Int64: 42, Valid: true}", dst.Count)
	}
	if !dst.Price.Valid || dst.Price.Float64 != 19.99 {
		t.Errorf("Price = %+v, want {Float64: 19.99, Valid: true}", dst.Price)
	}
	if !dst.Active {
		t.Errorf("Active = false, want true")
	}
	if !dst.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", dst.CreatedAt, now)
	}
}

func TestTypeAdapter_ConvertStruct_PartialFields(t *testing.T) {
	type Source struct {
		ID   int64
		Name string
	}
	type Dest struct {
		ID          int64
		Name        string
		Description string // Not in source
	}

	adapter := NewTypeAdapter()
	src := Source{ID: 999, Name: "partial"}
	dst := Dest{Description: "existing value"} // Pre-set value

	err := adapter.ConvertStruct(src, &dst)
	if err != nil {
		t.Fatalf("ConvertStruct failed: %v", err)
	}

	if dst.ID != 999 || dst.Name != "partial" {
		t.Errorf("Fields not copied. Got ID=%d, Name=%q", dst.ID, dst.Name)
	}
	if dst.Description != "existing value" {
		t.Errorf("Description was changed to %q, want %q", dst.Description, "existing value")
	}
}

func TestTypeAdapter_ConvertStruct_Pointers(t *testing.T) {
	type TestStruct struct {
		ID   int64
		Name string
	}

	adapter := NewTypeAdapter()
	src := &TestStruct{ID: 100, Name: "pointer test"}
	dst := &TestStruct{}

	err := adapter.ConvertStruct(src, dst)
	if err != nil {
		t.Fatalf("ConvertStruct failed: %v", err)
	}

	if dst.ID != 100 || dst.Name != "pointer test" {
		t.Errorf("Fields not copied. Got %+v", dst)
	}
}

func TestTypeAdapter_ConvertStruct_NonSettable(t *testing.T) {
	type TestStruct struct {
		ID int64
	}

	adapter := NewTypeAdapter()
	src := TestStruct{ID: 1}
	dst := TestStruct{} // Not a pointer - cannot set

	err := adapter.ConvertStruct(src, dst)
	if err == nil {
		t.Fatal("Expected error for non-settable destination, got nil")
	}
}

func TestTypeAdapter_ConvertStruct_NullStrings(t *testing.T) {
	type TestStruct struct {
		Name sql.NullString
	}

	adapter := NewTypeAdapter()
	src := TestStruct{Name: sql.NullString{String: "test", Valid: true}}
	var dst TestStruct

	err := adapter.ConvertStruct(src, &dst)
	if err != nil {
		t.Fatalf("ConvertStruct failed: %v", err)
	}

	if !dst.Name.Valid || dst.Name.String != "test" {
		t.Errorf("Name = %+v, want {String: 'test', Valid: true}", dst.Name)
	}
}

func TestTypeAdapter_ConvertStruct_NullBool(t *testing.T) {
	type TestStruct struct {
		Active sql.NullBool
	}

	adapter := NewTypeAdapter()
	src := TestStruct{Active: sql.NullBool{Bool: true, Valid: true}}
	var dst TestStruct

	err := adapter.ConvertStruct(src, &dst)
	if err != nil {
		t.Fatalf("ConvertStruct failed: %v", err)
	}

	if !dst.Active.Valid || !dst.Active.Bool {
		t.Errorf("Active = %+v, want {Bool: true, Valid: true}", dst.Active)
	}
}

func TestConvertSlice(t *testing.T) {
	type Source struct {
		ID int32
	}
	type Dest struct {
		ID int64
	}

	adapter := NewTypeAdapter()
	src := []Source{
		{ID: 1},
		{ID: 2},
		{ID: 3},
	}

	dst, err := ConvertSlice[Source, Dest](src, adapter)
	if err != nil {
		t.Fatalf("ConvertSlice failed: %v", err)
	}

	if len(dst) != 3 {
		t.Fatalf("len(dst) = %d, want 3", len(dst))
	}

	for i, d := range dst {
		if d.ID != int64(i+1) {
			t.Errorf("dst[%d].ID = %d, want %d", i, d.ID, i+1)
		}
	}
}

func TestConvertSlice_EmptySlice(t *testing.T) {
	type Source struct {
		ID int32
	}
	type Dest struct {
		ID int64
	}

	adapter := NewTypeAdapter()
	src := []Source{}

	dst, err := ConvertSlice[Source, Dest](src, adapter)
	if err != nil {
		t.Fatalf("ConvertSlice failed: %v", err)
	}

	if len(dst) != 0 {
		t.Errorf("len(dst) = %d, want 0", len(dst))
	}
}

func TestTypeAdapter_ConvertStruct_TimeHandling(t *testing.T) {
	type Source struct {
		CreatedAt time.Time
	}
	type Dest struct {
		CreatedAt time.Time
	}

	adapter := NewTypeAdapter()
	now := time.Now()
	src := Source{CreatedAt: now}
	var dst Dest

	err := adapter.ConvertStruct(src, &dst)
	if err != nil {
		t.Fatalf("ConvertStruct failed: %v", err)
	}

	if !dst.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", dst.CreatedAt, now)
	}
}

func TestTypeAdapter_ConvertStruct_NullTime(t *testing.T) {
	type TestStruct struct {
		UpdatedAt sql.NullTime
	}

	adapter := NewTypeAdapter()
	now := time.Now()
	src := TestStruct{UpdatedAt: sql.NullTime{Time: now, Valid: true}}
	var dst TestStruct

	err := adapter.ConvertStruct(src, &dst)
	if err != nil {
		t.Fatalf("ConvertStruct failed: %v", err)
	}

	if !dst.UpdatedAt.Valid || !dst.UpdatedAt.Time.Equal(now) {
		t.Errorf("UpdatedAt = %+v, want {Time: %v, Valid: true}", dst.UpdatedAt, now)
	}
}

// Benchmark tests
func BenchmarkTypeAdapter_ConvertStruct_Simple(b *testing.B) {
	type TestStruct struct {
		ID   int64
		Name string
	}

	adapter := NewTypeAdapter()
	src := TestStruct{ID: 123, Name: "benchmark"}
	var dst TestStruct

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = adapter.ConvertStruct(src, &dst)
	}
}

func BenchmarkTypeAdapter_ConvertStruct_Complex(b *testing.B) {
	type Source struct {
		ID        int32
		Name      string
		Count     sql.NullInt32
		Price     sql.NullFloat64
		Active    bool
		CreatedAt time.Time
	}
	type Dest struct {
		ID        int64
		Name      string
		Count     sql.NullInt64
		Price     sql.NullFloat64
		Active    bool
		CreatedAt time.Time
	}

	adapter := NewTypeAdapter()
	src := Source{
		ID:        123,
		Name:      "benchmark item",
		Count:     sql.NullInt32{Int32: 42, Valid: true},
		Price:     sql.NullFloat64{Float64: 19.99, Valid: true},
		Active:    true,
		CreatedAt: time.Now(),
	}
	var dst Dest

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = adapter.ConvertStruct(src, &dst)
	}
}
