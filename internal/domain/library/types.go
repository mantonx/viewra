package library

// LibraryType represents the type of media in a library
type LibraryType string

const (
	// LibraryTypeMovies represents a movie library
	LibraryTypeMovies LibraryType = "movies"

	// LibraryTypeTV represents a TV show library
	LibraryTypeTV LibraryType = "tv"

	// LibraryTypeMusic represents a music library
	LibraryTypeMusic LibraryType = "music"
)

// IsValid checks if the library type is valid
func (lt LibraryType) IsValid() bool {
	switch lt {
	case LibraryTypeMovies, LibraryTypeTV, LibraryTypeMusic:
		return true
	default:
		return false
	}
}

// String returns the string representation of the library type
func (lt LibraryType) String() string {
	return string(lt)
}
