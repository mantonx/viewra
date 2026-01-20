package internal

// TMDb API response types.
// These are pure data structures for JSON unmarshaling.

// MovieSearchResult represents a movie from TMDb search results.
type MovieSearchResult struct {
	ID               int     `json:"id"`
	Title            string  `json:"title"`
	OriginalTitle    string  `json:"original_title"`
	Overview         string  `json:"overview"`
	ReleaseDate      string  `json:"release_date"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	OriginalLanguage string  `json:"original_language"`
	Popularity       float64 `json:"popularity"`
}

// MovieSearchResponse is the response from TMDb movie search.
type MovieSearchResponse struct {
	Page         int                 `json:"page"`
	TotalPages   int                 `json:"total_pages"`
	TotalResults int                 `json:"total_results"`
	Results      []MovieSearchResult `json:"results"`
}

// MovieDetails contains full movie information from TMDb.
type MovieDetails struct {
	ID                  int                 `json:"id"`
	IMDbID              string              `json:"imdb_id"`
	Title               string              `json:"title"`
	OriginalTitle       string              `json:"original_title"`
	Overview            string              `json:"overview"`
	Tagline             string              `json:"tagline"`
	ReleaseDate         string              `json:"release_date"`
	Runtime             int                 `json:"runtime"`
	Budget              int64               `json:"budget"`
	Revenue             int64               `json:"revenue"`
	VoteAverage         float64             `json:"vote_average"`
	VoteCount           int                 `json:"vote_count"`
	PosterPath          string              `json:"poster_path"`
	BackdropPath        string              `json:"backdrop_path"`
	OriginalLanguage    string              `json:"original_language"`
	Genres              []Genre             `json:"genres"`
	ProductionCompanies []ProductionCompany `json:"production_companies"`
	ProductionCountries []ProductionCountry `json:"production_countries"`

	// Appended responses (when using append_to_response)
	Credits         *Credits                 `json:"credits,omitempty"`
	Images          *Images                  `json:"images,omitempty"`
	Keywords        *Keywords                `json:"keywords,omitempty"`
	Recommendations *RecommendationsResponse `json:"recommendations,omitempty"`
}

// TVSearchResult represents a TV show from TMDb search results.
type TVSearchResult struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	OriginalName     string  `json:"original_name"`
	Overview         string  `json:"overview"`
	FirstAirDate     string  `json:"first_air_date"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	OriginalLanguage string  `json:"original_language"`
	Popularity       float64 `json:"popularity"`
}

// TVSearchResponse is the response from TMDb TV search.
type TVSearchResponse struct {
	Page         int              `json:"page"`
	TotalPages   int              `json:"total_pages"`
	TotalResults int              `json:"total_results"`
	Results      []TVSearchResult `json:"results"`
}

// TVDetails contains full TV show information from TMDb.
type TVDetails struct {
	ID                  int                 `json:"id"`
	Name                string              `json:"name"`
	OriginalName        string              `json:"original_name"`
	Overview            string              `json:"overview"`
	Tagline             string              `json:"tagline"`
	FirstAirDate        string              `json:"first_air_date"`
	LastAirDate         string              `json:"last_air_date"`
	Status              string              `json:"status"`
	Type                string              `json:"type"`
	NumberOfSeasons     int                 `json:"number_of_seasons"`
	NumberOfEpisodes    int                 `json:"number_of_episodes"`
	EpisodeRunTime      []int               `json:"episode_run_time"`
	VoteAverage         float64             `json:"vote_average"`
	VoteCount           int                 `json:"vote_count"`
	PosterPath          string              `json:"poster_path"`
	BackdropPath        string              `json:"backdrop_path"`
	OriginalLanguage    string              `json:"original_language"`
	Genres              []Genre             `json:"genres"`
	Networks            []Network           `json:"networks"`
	ProductionCompanies []ProductionCompany `json:"production_companies"`
	CreatedBy           []Creator           `json:"created_by"`

	// External IDs (when using append_to_response)
	ExternalIDs     *ExternalIDs             `json:"external_ids,omitempty"`
	Credits         *Credits                 `json:"credits,omitempty"`
	Images          *Images                  `json:"images,omitempty"`
	Keywords        *Keywords                `json:"keywords,omitempty"`
	Recommendations *RecommendationsResponse `json:"recommendations,omitempty"`
}

// Genre represents a genre.
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ProductionCompany represents a production company/studio.
type ProductionCompany struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

// ProductionCountry represents a production country.
type ProductionCountry struct {
	ISO3166_1 string `json:"iso_3166_1"`
	Name      string `json:"name"`
}

// Network represents a TV network.
type Network struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

// Creator represents a TV show creator.
type Creator struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ProfilePath string `json:"profile_path"`
}

// Credits contains cast and crew information.
type Credits struct {
	Cast []CastMember `json:"cast"`
	Crew []CrewMember `json:"crew"`
}

// CastMember represents an actor in the cast.
type CastMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	Order       int    `json:"order"`
	ProfilePath string `json:"profile_path"`
}

// CrewMember represents a crew member.
type CrewMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
}

// Images contains image information.
type Images struct {
	Backdrops []Image `json:"backdrops"`
	Posters   []Image `json:"posters"`
	Logos     []Image `json:"logos"`
}

// Image represents an image from TMDb.
type Image struct {
	FilePath    string  `json:"file_path"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	AspectRatio float64 `json:"aspect_ratio"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
	ISO639_1    string  `json:"iso_639_1"`
}

// ExternalIDs contains external IDs for a TV show.
type ExternalIDs struct {
	IMDbID      string `json:"imdb_id"`
	TVDbID      int    `json:"tvdb_id"`
	FacebookID  string `json:"facebook_id"`
	InstagramID string `json:"instagram_id"`
	TwitterID   string `json:"twitter_id"`
}

// Keywords contains keyword information from TMDb.
type Keywords struct {
	Keywords []Keyword `json:"keywords"` // Used for movies
	Results  []Keyword `json:"results"`  // Used for TV shows
}

// Keyword represents a single keyword/tag from TMDb.
type Keyword struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// FindByExternalIDResponse is the response from TMDb find endpoint.
type FindByExternalIDResponse struct {
	MovieResults []MovieSearchResult `json:"movie_results"`
	TVResults    []TVSearchResult    `json:"tv_results"`
}

// RecommendationsResponse contains recommended similar titles from TMDb.
type RecommendationsResponse struct {
	Page         int                    `json:"page"`
	TotalPages   int                    `json:"total_pages"`
	TotalResults int                    `json:"total_results"`
	Results      []RecommendationResult `json:"results"`
}

// RecommendationResult represents a recommended movie or TV show.
type RecommendationResult struct {
	ID           int     `json:"id"`
	Title        string  `json:"title,omitempty"`         // Movie title
	Name         string  `json:"name,omitempty"`          // TV show name
	MediaType    string  `json:"media_type,omitempty"`    // "movie" or "tv"
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int     `json:"vote_count"`
	Popularity   float64 `json:"popularity"`
}

// TrendingResult represents an item from the TMDb trending endpoint.
// This is a union type that can be either a movie or TV show.
type TrendingResult struct {
	// Common fields
	ID               int     `json:"id"`
	MediaType        string  `json:"media_type"` // "movie" or "tv"
	Overview         string  `json:"overview"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	OriginalLanguage string  `json:"original_language"`
	Popularity       float64 `json:"popularity"`

	// Movie-specific fields
	Title         string `json:"title,omitempty"`
	OriginalTitle string `json:"original_title,omitempty"`
	ReleaseDate   string `json:"release_date,omitempty"`

	// TV-specific fields
	Name         string `json:"name,omitempty"`
	OriginalName string `json:"original_name,omitempty"`
	FirstAirDate string `json:"first_air_date,omitempty"`
}

// TrendingResponse is the response from TMDb trending endpoint.
type TrendingResponse struct {
	Page         int              `json:"page"`
	TotalPages   int              `json:"total_pages"`
	TotalResults int              `json:"total_results"`
	Results      []TrendingResult `json:"results"`
}
