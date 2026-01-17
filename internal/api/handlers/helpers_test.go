package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

func TestParseFloat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"valid float", "3.14", 3.14, false},
		{"valid integer as float", "42", 42.0, false},
		{"valid negative float", "-1.5", -1.5, false},
		{"valid zero", "0", 0.0, false},
		{"valid scientific notation", "1e10", 1e10, false},
		{"invalid non-numeric", "abc", 0, true},
		{"invalid empty string", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFloat(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFloat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseFloat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseIDList(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []int64
		wantErr bool
	}{
		{"empty string", "", []int64{}, false},
		{"single ID", "123", []int64{123}, false},
		{"multiple IDs", "1,2,3", []int64{1, 2, 3}, false},
		{"IDs with spaces", "1, 2, 3", []int64{1, 2, 3}, false},
		{"invalid ID in list", "1,abc,3", nil, true},
		{"all invalid", "abc,def", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIDList(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIDList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("parseIDList() length = %d, want %d", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("parseIDList()[%d] = %v, want %v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string", "", []string{}},
		{"single value", "foo", []string{"foo"}},
		{"multiple values", "foo,bar,baz", []string{"foo", "bar", "baz"}},
		{"values with spaces", "foo, bar, baz", []string{"foo", "bar", "baz"}},
		{"empty values filtered", "foo,,bar", []string{"foo", "bar"}},
		{"whitespace only filtered", "foo,   ,bar", []string{"foo", "bar"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitCSV(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitCSV() length = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitCSV()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseCommaSeparatedHeader(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   []string
	}{
		{"empty header", "", nil},
		{"single value", "gzip", []string{"gzip"}},
		{"multiple values", "gzip, deflate, br", []string{"gzip", "deflate", "br"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCommaSeparatedHeader(tt.header)
			if tt.want == nil && got != nil {
				t.Errorf("parseCommaSeparatedHeader() = %v, want nil", got)
				return
			}
			if tt.want != nil {
				if len(got) != len(tt.want) {
					t.Errorf("parseCommaSeparatedHeader() length = %d, want %d", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("parseCommaSeparatedHeader()[%d] = %q, want %q", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name string
		time time.Time
		want string
	}{
		{
			name: "zero time returns empty",
			time: time.Time{},
			want: "",
		},
		{
			name: "valid time formatted as RFC3339",
			time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			want: "2024-01-15T10:30:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTime(tt.time)
			if got != tt.want {
				t.Errorf("formatTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetCurrentUserID(t *testing.T) {
	// In single-user mode, this should always return 1
	got := getCurrentUserID()
	if got != 1 {
		t.Errorf("getCurrentUserID() = %d, want 1", got)
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("with user_id in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", "42")

		got := getUserIDFromContext(c)
		if got != "42" {
			t.Errorf("getUserIDFromContext() = %q, want %q", got, "42")
		}
	})

	t.Run("without user_id in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		got := getUserIDFromContext(c)
		if got != "1" {
			t.Errorf("getUserIDFromContext() = %q, want %q (default)", got, "1")
		}
	})

	t.Run("with non-string user_id in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", 42) // int, not string

		got := getUserIDFromContext(c)
		if got != "1" {
			t.Errorf("getUserIDFromContext() = %q, want %q (default for non-string)", got, "1")
		}
	})
}

func TestParsePaginationParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
	}{
		{"default values", "", 50, 0},
		{"custom limit", "limit=20", 20, 0},
		{"custom offset", "offset=100", 50, 100},
		{"both custom", "limit=10&offset=50", 10, 50},
		{"invalid limit uses default", "limit=abc", 50, 0},
		{"invalid offset uses default", "offset=abc", 50, 0},
		{"negative limit uses default", "limit=-5", 50, 0},
		{"negative offset uses default", "offset=-10", 50, 0},
		{"zero limit uses default", "limit=0", 50, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req, _ := http.NewRequest("GET", "/?"+tt.query, nil)
			c.Request = req

			got := parsePaginationParams(c)
			if got.limit != tt.wantLimit {
				t.Errorf("parsePaginationParams().limit = %d, want %d", got.limit, tt.wantLimit)
			}
			if got.offset != tt.wantOffset {
				t.Errorf("parsePaginationParams().offset = %d, want %d", got.offset, tt.wantOffset)
			}
		})
	}
}

func TestGetRequiredQueryInt64(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		paramName  string
		wantValue  int64
		wantOK     bool
		wantStatus int
	}{
		{"valid param", "library_id=123", "library_id", 123, true, 0},
		{"missing param", "", "library_id", 0, false, http.StatusBadRequest},
		{"invalid param", "library_id=abc", "library_id", 0, false, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req, _ := http.NewRequest("GET", "/?"+tt.query, nil)
			c.Request = req

			got, ok := getRequiredQueryInt64(c, tt.paramName)
			if ok != tt.wantOK {
				t.Errorf("getRequiredQueryInt64() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantValue {
				t.Errorf("getRequiredQueryInt64() value = %d, want %d", got, tt.wantValue)
			}
			if !tt.wantOK && w.Code != tt.wantStatus {
				t.Errorf("getRequiredQueryInt64() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestGetOptionalQueryInt64(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		query     string
		paramName string
		wantValue int64
		wantOK    bool
	}{
		{"valid param", "media_id=456", "media_id", 456, true},
		{"missing param", "", "media_id", 0, false},
		{"invalid param", "media_id=xyz", "media_id", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req, _ := http.NewRequest("GET", "/?"+tt.query, nil)
			c.Request = req

			got, ok := getOptionalQueryInt64(c, tt.paramName)
			if ok != tt.wantOK {
				t.Errorf("getOptionalQueryInt64() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantValue {
				t.Errorf("getOptionalQueryInt64() value = %d, want %d", got, tt.wantValue)
			}
		})
	}
}

func TestGetPathInt64(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		paramValue string
		wantValue  int64
		wantOK     bool
	}{
		{"valid param", "789", 789, true},
		{"invalid param", "abc", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: tt.paramValue}}

			got, ok := getPathInt64(c, "id")
			if ok != tt.wantOK {
				t.Errorf("getPathInt64() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantValue {
				t.Errorf("getPathInt64() value = %d, want %d", got, tt.wantValue)
			}
		})
	}
}

func TestGetQueryInt(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		query        string
		paramName    string
		defaultValue int
		want         int
	}{
		{"valid param", "page=5", "page", 1, 5},
		{"missing param uses default", "", "page", 1, 1},
		{"invalid param uses default", "page=abc", "page", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req, _ := http.NewRequest("GET", "/?"+tt.query, nil)
			c.Request = req

			got := getQueryInt(c, tt.paramName, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getQueryInt() = %d, want %d", got, tt.want)
			}
		})
	}
}
