package querier

import (
	"reflect"
	"testing"
)

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty string", "", nil},
		{"single value", "value", []string{"value"}},
		{"multiple values", "a,b,c", []string{"a", "b", "c"}},
		{"with spaces", " a , b , c ", []string{"a", "b", "c"}},
		{"empty values", "a,,b,", []string{"a", "b"}},
		{"only spaces", " , , ", []string{}},
		{"mixed", "one, two,  three  ", []string{"one", "two", "three"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndTrim(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("splitAndTrim(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseCastString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []CastMemberInfo
	}{
		{"empty string", "", nil},
		{"single actor", "John Doe", []CastMemberInfo{{Name: "John Doe", Order: 0}}},
		{"multiple actors", "John Doe, Jane Smith, Bob Wilson", []CastMemberInfo{
			{Name: "John Doe", Order: 0},
			{Name: "Jane Smith", Order: 1},
			{Name: "Bob Wilson", Order: 2},
		}},
		{"with extra spaces", " John Doe , Jane Smith ", []CastMemberInfo{
			{Name: "John Doe", Order: 0},
			{Name: "Jane Smith", Order: 1},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCastString(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parseCastString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeLanguageCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Empty/whitespace
		{"empty string", "", ""},
		{"whitespace only", "  ", ""},

		// ISO 639-2 (3-letter) to ISO 639-1 (2-letter)
		{"korean 3-letter", "kor", "ko"},
		{"japanese 3-letter", "jpn", "ja"},
		{"english 3-letter", "eng", "en"},
		{"chinese zho", "zho", "zh"},
		{"chinese chi", "chi", "zh"},
		{"chinese cmn", "cmn", "zh"},
		{"spanish", "spa", "es"},
		{"french fra", "fra", "fr"},
		{"french fre", "fre", "fr"},
		{"german deu", "deu", "de"},
		{"german ger", "ger", "de"},
		{"italian", "ita", "it"},
		{"portuguese", "por", "pt"},
		{"russian", "rus", "ru"},
		{"hindi", "hin", "hi"},
		{"arabic", "ara", "ar"},
		{"thai", "tha", "th"},
		{"vietnamese", "vie", "vi"},
		{"indonesian", "ind", "id"},
		{"swedish", "swe", "sv"},
		{"norwegian", "nor", "no"},
		{"danish", "dan", "da"},
		{"finnish", "fin", "fi"},
		{"dutch dut", "dut", "nl"},
		{"dutch nld", "nld", "nl"},
		{"polish", "pol", "pl"},
		{"turkish", "tur", "tr"},
		{"greek gre", "gre", "el"},
		{"greek ell", "ell", "el"},
		{"hebrew", "heb", "he"},
		{"persian per", "per", "fa"},
		{"persian fas", "fas", "fa"},
		{"tamil", "tam", "ta"},
		{"telugu", "tel", "te"},
		{"malayalam", "mal", "ml"},
		{"kannada", "kan", "kn"},
		{"marathi", "mar", "mr"},
		{"bengali", "ben", "bn"},
		{"punjabi pun", "pun", "pa"},
		{"punjabi pan", "pan", "pa"},
		{"gujarati", "guj", "gu"},

		// Full language names
		{"korean full", "korean", "ko"},
		{"japanese full", "japanese", "ja"},
		{"english full", "english", "en"},
		{"chinese full", "chinese", "zh"},
		{"spanish full", "spanish", "es"},
		{"french full", "french", "fr"},
		{"german full", "german", "de"},

		// ISO 639-1 (2-letter) passthrough
		{"english 2-letter", "en", "en"},
		{"korean 2-letter", "ko", "ko"},
		{"japanese 2-letter", "ja", "ja"},
		{"french 2-letter", "fr", "fr"},

		// Case insensitivity
		{"uppercase", "ENG", "en"},
		{"mixed case", "Kor", "ko"},
		{"mixed case full", "Korean", "ko"},

		// With whitespace
		{"with leading space", " eng", "en"},
		{"with trailing space", "eng ", "en"},
		{"with both spaces", " eng ", "en"},

		// Unknown codes
		{"unknown 3-letter", "xxx", ""},
		{"unknown 4-letter", "xxxx", ""},
		{"unknown word", "unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeLanguageCode(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeLanguageCode(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
