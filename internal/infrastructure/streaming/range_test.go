package streaming

import (
	"testing"
)

func TestParseRangeHeader(t *testing.T) {
	tests := []struct {
		name        string
		rangeHeader string
		fileSize    int64
		want        []ByteRange
		wantErr     bool
	}{
		// Valid single ranges
		{
			name:        "Normal range",
			rangeHeader: "bytes=0-499",
			fileSize:    1000,
			want:        []ByteRange{{Start: 0, End: 499}},
			wantErr:     false,
		},
		{
			name:        "Open-ended range",
			rangeHeader: "bytes=500-",
			fileSize:    1000,
			want:        []ByteRange{{Start: 500, End: 999}},
			wantErr:     false,
		},
		{
			name:        "Suffix range",
			rangeHeader: "bytes=-500",
			fileSize:    1000,
			want:        []ByteRange{{Start: 500, End: 999}},
			wantErr:     false,
		},
		{
			name:        "Suffix range larger than file",
			rangeHeader: "bytes=-2000",
			fileSize:    1000,
			want:        []ByteRange{{Start: 0, End: 999}},
			wantErr:     false,
		},
		{
			name:        "Full file range",
			rangeHeader: "bytes=0-",
			fileSize:    1000,
			want:        []ByteRange{{Start: 0, End: 999}},
			wantErr:     false,
		},
		{
			name:        "Range to end of file",
			rangeHeader: "bytes=0-999",
			fileSize:    1000,
			want:        []ByteRange{{Start: 0, End: 999}},
			wantErr:     false,
		},
		{
			name:        "Range beyond file size adjusted",
			rangeHeader: "bytes=0-5000",
			fileSize:    1000,
			want:        []ByteRange{{Start: 0, End: 999}},
			wantErr:     false,
		},
		{
			name:        "Single byte range",
			rangeHeader: "bytes=0-0",
			fileSize:    1000,
			want:        []ByteRange{{Start: 0, End: 0}},
			wantErr:     false,
		},
		{
			name:        "Last byte range",
			rangeHeader: "bytes=-1",
			fileSize:    1000,
			want:        []ByteRange{{Start: 999, End: 999}},
			wantErr:     false,
		},

		// Multiple ranges (for future support)
		{
			name:        "Multiple ranges",
			rangeHeader: "bytes=0-499,500-999",
			fileSize:    1000,
			want: []ByteRange{
				{Start: 0, End: 499},
				{Start: 500, End: 999},
			},
			wantErr: false,
		},

		// Invalid ranges
		{
			name:        "Invalid prefix",
			rangeHeader: "octets=0-499",
			fileSize:    1000,
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "Missing bytes prefix",
			rangeHeader: "0-499",
			fileSize:    1000,
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "No dash",
			rangeHeader: "bytes=0",
			fileSize:    1000,
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "Empty range",
			rangeHeader: "bytes=-",
			fileSize:    1000,
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "Start beyond file size",
			rangeHeader: "bytes=1000-1500",
			fileSize:    1000,
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "End before start",
			rangeHeader: "bytes=500-100",
			fileSize:    1000,
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "Invalid start position",
			rangeHeader: "bytes=abc-500",
			fileSize:    1000,
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "Invalid end position",
			rangeHeader: "bytes=0-xyz",
			fileSize:    1000,
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "Invalid suffix length",
			rangeHeader: "bytes=-abc",
			fileSize:    1000,
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "Negative start",
			rangeHeader: "bytes=-100-500",
			fileSize:    1000,
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "Empty range spec",
			rangeHeader: "bytes=",
			fileSize:    1000,
			want:        nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRangeHeader(tt.rangeHeader, tt.fileSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRangeHeader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("ParseRangeHeader() got %d ranges, want %d", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i].Start != tt.want[i].Start || got[i].End != tt.want[i].End {
						t.Errorf("ParseRangeHeader() range[%d] = {%d, %d}, want {%d, %d}",
							i, got[i].Start, got[i].End, tt.want[i].Start, tt.want[i].End)
					}
				}
			}
		})
	}
}

func TestParseRangePart(t *testing.T) {
	tests := []struct {
		name     string
		part     string
		fileSize int64
		want     ByteRange
		wantErr  bool
	}{
		{
			name:     "Normal range",
			part:     "0-499",
			fileSize: 1000,
			want:     ByteRange{Start: 0, End: 499},
			wantErr:  false,
		},
		{
			name:     "Open-ended range",
			part:     "500-",
			fileSize: 1000,
			want:     ByteRange{Start: 500, End: 999},
			wantErr:  false,
		},
		{
			name:     "Suffix range",
			part:     "-500",
			fileSize: 1000,
			want:     ByteRange{Start: 500, End: 999},
			wantErr:  false,
		},
		{
			name:     "Invalid format no dash",
			part:     "500",
			fileSize: 1000,
			want:     ByteRange{},
			wantErr:  true,
		},
		{
			name:     "Empty range",
			part:     "-",
			fileSize: 1000,
			want:     ByteRange{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRangePart(tt.part, tt.fileSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRangePart() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && (got.Start != tt.want.Start || got.End != tt.want.End) {
				t.Errorf("parseRangePart() = {%d, %d}, want {%d, %d}",
					got.Start, got.End, tt.want.Start, tt.want.End)
			}
		})
	}
}
