package types

// TMDb API response types
type SearchResponse struct {
	Page         int      `json:"page"`
	Results      []Result `json:"results"`
	TotalPages   int      `json:"total_pages"`
	TotalResults int      `json:"total_results"`
}

type Result struct {
	ID               int      `json:"id"`
	Title            string   `json:"title,omitempty"`          // Movies
	Name             string   `json:"name,omitempty"`           // TV Shows
	OriginalTitle    string   `json:"original_title,omitempty"` // Movies
	OriginalName     string   `json:"original_name,omitempty"`  // TV Shows
	Overview         string   `json:"overview"`
	ReleaseDate      string   `json:"release_date,omitempty"`   // Movies
	FirstAirDate     string   `json:"first_air_date,omitempty"` // TV Shows
	GenreIDs         []int    `json:"genre_ids"`
	VoteAverage      float64  `json:"vote_average"`
	VoteCount        int      `json:"vote_count"`
	Popularity       float64  `json:"popularity"`
	PosterPath       string   `json:"poster_path,omitempty"`
	BackdropPath     string   `json:"backdrop_path,omitempty"`
	Adult            bool     `json:"adult,omitempty"`
	Video            bool     `json:"video,omitempty"`
	MediaType        string   `json:"media_type,omitempty"`
	OriginCountry    []string `json:"origin_country,omitempty"` // TV Shows
	OriginalLanguage string   `json:"original_language"`
}

// TMDb Images API response types
type ImagesResponse struct {
	ID        int         `json:"id"`
	Backdrops []ImageInfo `json:"backdrops"`
	Logos     []ImageInfo `json:"logos"`
	Posters   []ImageInfo `json:"posters"`
	Stills    []ImageInfo `json:"stills,omitempty"` // For episodes
}

type ImageInfo struct {
	AspectRatio float64 `json:"aspect_ratio"`
	Height      int     `json:"height"`
	FilePath    string  `json:"file_path"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
	Width       int     `json:"width"`
	ISO639_1    string  `json:"iso_639_1,omitempty"` // Language code
}

// TV Season details response
type TVSeasonDetails struct {
	ID           int                `json:"id"`
	Name         string             `json:"name"`
	Overview     string             `json:"overview"`
	PosterPath   string             `json:"poster_path"`
	SeasonNumber int                `json:"season_number"`
	AirDate      string             `json:"air_date"`
	Episodes     []TVEpisodeDetails `json:"episodes"`
	Images       *ImagesResponse    `json:"images,omitempty"`
}

// TV Episode details response
type TVEpisodeDetails struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	AirDate       string  `json:"air_date"`
	EpisodeNumber int     `json:"episode_number"`
	SeasonNumber  int     `json:"season_number"`
	StillPath     string  `json:"still_path"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
}

// Movie details response
type MovieDetails struct {
	ID               int     `json:"id"`
	Title            string  `json:"title"`
	OriginalTitle    string  `json:"original_title"`
	Overview         string  `json:"overview"`
	Tagline          string  `json:"tagline"`
	ReleaseDate      string  `json:"release_date"`
	Runtime          int     `json:"runtime"`
	Status           string  `json:"status"`
	Adult            bool    `json:"adult"`
	Video            bool    `json:"video"`
	Homepage         string  `json:"homepage"`
	Budget           int64   `json:"budget"`
	Revenue          int64   `json:"revenue"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	Popularity       float64 `json:"popularity"`
	OriginalLanguage string  `json:"original_language"`
	Genres           []Genre `json:"genres"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
}

// TV Series details response
type TVSeriesDetails struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	OriginalName     string   `json:"original_name"`
	Overview         string   `json:"overview"`
	FirstAirDate     string   `json:"first_air_date"`
	LastAirDate      string   `json:"last_air_date"`
	Status           string   `json:"status"`
	Type             string   `json:"type"`
	InProduction     bool     `json:"in_production"`
	NumberOfSeasons  int      `json:"number_of_seasons"`
	NumberOfEpisodes int      `json:"number_of_episodes"`
	EpisodeRunTime   []int    `json:"episode_run_time"`
	Genres           []Genre  `json:"genres"`
	VoteAverage      float64  `json:"vote_average"`
	VoteCount        int      `json:"vote_count"`
	Popularity       float64  `json:"popularity"`
	PosterPath       string   `json:"poster_path"`
	BackdropPath     string   `json:"backdrop_path"`
	OriginCountry    []string `json:"origin_country"`
	OriginalLanguage string   `json:"original_language"`
}

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
