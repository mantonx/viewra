package handlers

import (
	"testing"
)

func TestParseID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{
			name:    "valid positive ID",
			input:   "123",
			want:    123,
			wantErr: false,
		},
		{
			name:    "valid ID = 1",
			input:   "1",
			want:    1,
			wantErr: false,
		},
		{
			name:    "large valid ID",
			input:   "9223372036854775807", // Max int64
			want:    9223372036854775807,
			wantErr: false,
		},
		{
			name:    "invalid non-numeric",
			input:   "abc",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid empty string",
			input:   "",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid decimal",
			input:   "123.45",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid mixed alphanumeric",
			input:   "123abc",
			want:    0,
			wantErr: true,
		},
		{
			name:    "zero is valid",
			input:   "0",
			want:    0,
			wantErr: false,
		},
		{
			name:    "negative number is valid",
			input:   "-1",
			want:    -1,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{
			name:    "valid positive int",
			input:   "42",
			want:    42,
			wantErr: false,
		},
		{
			name:    "valid int = 0",
			input:   "0",
			want:    0,
			wantErr: false,
		},
		{
			name:    "valid negative int",
			input:   "-100",
			want:    -100,
			wantErr: false,
		},
		{
			name:    "invalid non-numeric",
			input:   "xyz",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid empty string",
			input:   "",
			want:    0,
			wantErr: true,
		},
		{
			name:    "decimal truncated to int",
			input:   "99.9",
			want:    99, // fmt.Sscanf parses this as 99, stopping at the decimal
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInt(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseInt() = %v, want %v", got, tt.want)
			}
		})
	}
}
