package host

import (
	"testing"
)

func TestVectorToBytes(t *testing.T) {
	tests := []struct {
		name   string
		vector []float32
	}{
		{"empty", []float32{}},
		{"single", []float32{1.0}},
		{"small", []float32{1.0, 2.0, 3.0}},
		{"negative", []float32{-1.0, 0.0, 1.0}},
		{"decimal", []float32{0.1, 0.2, 0.3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes := vectorToBytes(tt.vector)
			result := bytesToVector(bytes)

			if len(result) != len(tt.vector) {
				t.Errorf("length mismatch: got %d, want %d", len(result), len(tt.vector))
				return
			}

			for i := range result {
				if result[i] != tt.vector[i] {
					t.Errorf("value mismatch at %d: got %f, want %f", i, result[i], tt.vector[i])
				}
			}
		})
	}
}

func TestVectorToPostgresString(t *testing.T) {
	tests := []struct {
		name     string
		vector   []float32
		expected string
	}{
		{"empty", []float32{}, "[]"},
		{"single", []float32{1.0}, "[1]"},
		{"multiple", []float32{1.0, 2.0, 3.0}, "[1,2,3]"},
		{"negative", []float32{-1.0, 0.0, 1.0}, "[-1,0,1]"},
		{"decimal", []float32{0.5, 1.5, 2.5}, "[0.5,1.5,2.5]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vectorToPostgresString(tt.vector)
			if result != tt.expected {
				t.Errorf("vectorToPostgresString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestPostgresStringToVector(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []float32
	}{
		{"empty", "[]", nil},
		{"single", "[1]", []float32{1.0}},
		{"multiple", "[1,2,3]", []float32{1.0, 2.0, 3.0}},
		{"with spaces", "[1, 2, 3]", []float32{1.0, 2.0, 3.0}},
		{"negative", "[-1,0,1]", []float32{-1.0, 0.0, 1.0}},
		{"decimal", "[0.5,1.5,2.5]", []float32{0.5, 1.5, 2.5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := postgresStringToVector(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(result), len(tt.expected))
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("value mismatch at %d: got %f, want %f", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
		epsilon  float32
	}{
		{"identical", []float32{1, 0, 0}, []float32{1, 0, 0}, 1.0, 0.001},
		{"orthogonal", []float32{1, 0, 0}, []float32{0, 1, 0}, 0.0, 0.001},
		{"opposite", []float32{1, 0, 0}, []float32{-1, 0, 0}, -1.0, 0.001},
		{"similar", []float32{1, 1, 0}, []float32{1, 0, 0}, 0.707, 0.01},
		{"empty a", []float32{}, []float32{1, 0, 0}, 0.0, 0.001},
		{"empty b", []float32{1, 0, 0}, []float32{}, 0.0, 0.001},
		{"length mismatch", []float32{1, 0}, []float32{1, 0, 0}, 0.0, 0.001},
		{"zero a", []float32{0, 0, 0}, []float32{1, 0, 0}, 0.0, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.epsilon {
				t.Errorf("cosineSimilarity() = %f, want %f (epsilon %f)", result, tt.expected, tt.epsilon)
			}
		})
	}
}

func TestBytesToVector_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []float32
	}{
		{"nil", nil, nil},
		{"empty", []byte{}, nil},
		{"partial bytes", []byte{0x00, 0x01}, []float32{}}, // less than 4 bytes
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bytesToVector(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(result), len(tt.expected))
			}
		})
	}
}
