package library_test

import (
	"testing"

	"github.com/viewra/viewra/internal/domain/library"
)

func TestLibrary_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		library *library.Library
		wantErr error
	}{
		{
			name: "valid library",
			library: &library.Library{
				Name: "My Movies",
				Path: "/media/movies",
				Type: library.LibraryTypeMovies,
			},
			wantErr: nil,
		},
		{
			name: "empty name",
			library: &library.Library{
				Name: "",
				Path: "/media/movies",
				Type: library.LibraryTypeMovies,
			},
			wantErr: library.ErrInvalidName,
		},
		{
			name: "name too long",
			library: &library.Library{
				Name: string(make([]byte, 101)),
				Path: "/media/movies",
				Type: library.LibraryTypeMovies,
			},
			wantErr: library.ErrNameTooLong,
		},
		{
			name: "empty path",
			library: &library.Library{
				Name: "Movies",
				Path: "",
				Type: library.LibraryTypeMovies,
			},
			wantErr: library.ErrInvalidPath,
		},
		{
			name: "relative path",
			library: &library.Library{
				Name: "Movies",
				Path: "media/movies",
				Type: library.LibraryTypeMovies,
			},
			wantErr: library.ErrPathNotAbsolute,
		},
		{
			name: "path with parent reference",
			library: &library.Library{
				Name: "Movies",
				Path: "/media/movies/..",
				Type: library.LibraryTypeMovies,
			},
			wantErr: nil, // Actually valid - cleans to /media
		},
		{
			name: "invalid type",
			library: &library.Library{
				Name: "Movies",
				Path: "/media/movies",
				Type: "invalid",
			},
			wantErr: library.ErrInvalidType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.library.IsValid()
			
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				
				if err != tt.wantErr {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestLibraryType_IsValid(t *testing.T) {
	tests := []struct {
		name       string
		libType    library.LibraryType
		wantValid bool
	}{
		{
			name:       "movies type",
			libType:    library.LibraryTypeMovies,
			wantValid: true,
		},
		{
			name:       "tv type",
			libType:    library.LibraryTypeTV,
			wantValid: true,
		},
		{
			name:       "music type",
			libType:    library.LibraryTypeMusic,
			wantValid: true,
		},
		{
			name:       "invalid type",
			libType:    "invalid",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.libType.IsValid(); got != tt.wantValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal name",
			input: "My Movies",
			want:  "My Movies",
		},
		{
			name:  "name with whitespace",
			input: "  My Movies  ",
			want:  "My Movies",
		},
		{
			name:  "name too long",
			input: string(make([]byte, 150)),
			want:  string(make([]byte, 100)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := library.SanitizeName(tt.input); got != tt.want {
				t.Errorf("SanitizeName() = %q, want %q", got, tt.want)
			}
		})
	}
}
