package usecases

import (
	"log/slog"

	"github.com/mantonx/viewra/internal/app/config"
	"github.com/mantonx/viewra/internal/app/repositories"
	"github.com/mantonx/viewra/internal/app/services"
	"github.com/mantonx/viewra/internal/application/common"
	"github.com/mantonx/viewra/internal/application/images"
	"github.com/mantonx/viewra/internal/application/library"
	"github.com/mantonx/viewra/internal/application/media"
	"github.com/mantonx/viewra/internal/application/movies"
	"github.com/mantonx/viewra/internal/application/music"
	"github.com/mantonx/viewra/internal/application/transcode"
	"github.com/mantonx/viewra/internal/application/tv"
)

// UseCases holds all application use cases organized by domain.
// Groups related use cases for each major feature area of the application.
type UseCases struct {
	Library   *LibraryUseCases
	Media     *MediaUseCases
	Movies    *MovieUseCases
	TV        *TVUseCases
	Music     *MusicUseCases
	Images    *ImageUseCases
	Transcode *TranscodeUseCases
}

// LibraryUseCases holds library-related use cases
type LibraryUseCases struct {
	Create *library.CreateLibraryUseCase
	Update *library.UpdateLibraryUseCase
	Delete *library.DeleteLibraryUseCase
	Get    *library.GetLibraryUseCase
	List   *library.ListLibrariesUseCase
	Scan   *library.ScanLibraryUseCase
}

// MediaUseCases holds media-related use cases
type MediaUseCases struct {
	Get    *media.GetMediaUseCase
	List   *media.ListMediaUseCase
	Delete *media.DeleteMediaUseCase
}

// MovieUseCases holds movie-related use cases
type MovieUseCases struct {
	List    *movies.ListMoviesUseCase
	Get     *movies.GetMovieUseCase
	Search  *movies.SearchMoviesUseCase
	ListIDs *movies.ListMovieIDsUseCase
}

// TVUseCases holds TV show-related use cases
type TVUseCases struct {
	ListShows      *tv.ListTVShowsUseCase
	GetShow        *tv.GetTVShowUseCase
	ListEpisodes   *tv.ListTVEpisodesUseCase
	GetEpisode     *tv.GetTVEpisodeUseCase
	SearchEpisodes *tv.SearchTVEpisodesUseCase
	ListShowIDs    *tv.ListTVShowIDsUseCase
	GetNextEpisode *tv.GetNextEpisodeUseCase
}

// MusicUseCases holds music-related use cases
type MusicUseCases struct {
	ListArtists        *music.ListArtistsUseCase
	ListAlbumsByArtist *music.ListAlbumsByArtistIDUseCase
	ListTracksByAlbum  *music.ListTracksByAlbumIDUseCase
	GetTrack           *music.GetTrackUseCase
	SearchTracks       *music.SearchTracksUseCase
	ListArtistIDs      *music.ListArtistIDsUseCase
}

// ImageUseCases holds image-related use cases
type ImageUseCases struct {
	Get         *images.GetImageUseCase
	GetMedia    *images.GetMediaImagesUseCase
	GetEntity   *images.GetEntityImagesUseCase
	GetBatch    *images.GetBatchMediaImagesUseCase
	Cleanup     *images.CleanupUseCase
	ExtractMovie   *images.ExtractMovieImagesUseCase
	ExtractEpisode *images.ExtractTVEpisodeImagesUseCase
	ExtractShow    *images.ExtractTVShowImagesUseCase
	ExtractSeason  *images.ExtractTVSeasonImagesUseCase
	ExtractAlbum   *images.ExtractMusicAlbumImagesUseCase
	ExtractArtist  *images.ExtractMusicArtistImagesUseCase
}

// TranscodeUseCases holds transcoding-related use cases
type TranscodeUseCases struct {
	CreateJob     *transcode.CreateJobUseCase
	GetStatus     *transcode.GetJobStatusUseCase
	ServeManifest *transcode.ServeManifestUseCase
}

// BuildUseCases creates and wires all use case instances grouped by domain.
// Delegates to domain-specific builder functions for each feature area.
func BuildUseCases(
	cfg *config.Config,
	repos *repositories.Repositories,
	svcs *services.Services,
	txManager *common.TxManager,
	logger *slog.Logger,
) *UseCases {
	return &UseCases{
		Library:   buildLibraryUseCases(repos, svcs, txManager, logger, cfg),
		Media:     buildMediaUseCases(repos, svcs, txManager, logger, cfg.Images.CacheDir),
		Movies:    buildMovieUseCases(repos),
		TV:        buildTVUseCases(repos),
		Music:     buildMusicUseCases(repos),
		Images:    buildImageUseCases(repos, svcs, logger, cfg.Images.CacheDir),
		Transcode: buildTranscodeUseCases(repos, svcs),
	}
}

// buildLibraryUseCases creates library use cases
func buildLibraryUseCases(
	repos *repositories.Repositories,
	svcs *services.Services,
	txManager *common.TxManager,
	logger *slog.Logger,
	cfg *config.Config,
) *LibraryUseCases {
	// Image cleanup use case (needed for library delete and scan)
	imageCleanup := images.NewCleanupUseCase(repos.Image, cfg.Images.CacheDir, logger)

	// Image extraction use cases (needed for scanner)
	extractMovie := images.NewExtractMovieImagesUseCase(repos.Image, svcs.ImageCache, svcs.ImageTransformer)
	extractEpisode := images.NewExtractTVEpisodeImagesUseCase(repos.Image, svcs.ImageCache, svcs.ImageTransformer)
	extractShow := images.NewExtractTVShowImagesUseCase(repos.Image, svcs.ImageCache, svcs.ImageTransformer)
	extractSeason := images.NewExtractTVSeasonImagesUseCase(repos.Image, svcs.ImageCache, svcs.ImageTransformer)
	extractAlbum := images.NewExtractMusicAlbumImagesUseCase(repos.Image, svcs.ImageCache, svcs.ImageTransformer)
	extractArtist := images.NewExtractMusicArtistImagesUseCase(repos.Image, svcs.ImageCache, svcs.ImageTransformer)

	return &LibraryUseCases{
		Create: library.NewCreateLibraryUseCase(repos.Library, txManager),
		Update: library.NewUpdateLibraryUseCase(repos.Library),
		Delete: library.NewDeleteLibraryUseCase(repos.Library, repos.Image, imageCleanup, txManager),
		Get:    library.NewGetLibraryUseCase(repos.Library),
		List:   library.NewListLibrariesUseCase(repos.Library),
		Scan: library.NewScanLibraryUseCase(
			repos.Library,
			repos.Media,
			repos.Movie,
			repos.TV,
			repos.Music,
			repos.ScanJob,
			repos.Checkpoint,
			repos.ScanState,
			extractMovie,
			extractEpisode,
			extractShow,
			extractSeason,
			extractAlbum,
			extractArtist,
			repos.Image,
			imageCleanup,
			cfg.Media.ScanTimeout,
			cfg.SystemProfile,
			logger,
		),
	}
}

// buildMediaUseCases creates media use cases
func buildMediaUseCases(
	repos *repositories.Repositories,
	svcs *services.Services,
	txManager *common.TxManager,
	logger *slog.Logger,
	imageCacheDir string,
) *MediaUseCases {
	// Image cleanup is needed for deleteMedia but we'll create it locally
	// to avoid circular dependencies
	imageCleanup := images.NewCleanupUseCase(repos.Image, imageCacheDir, logger)

	return &MediaUseCases{
		Get:    media.NewGetMediaUseCase(repos.Media),
		List:   media.NewListMediaUseCase(repos.Media),
		Delete: media.NewDeleteMediaUseCase(repos.Media, repos.Image, imageCleanup),
	}
}

// buildMovieUseCases creates movie use cases
func buildMovieUseCases(repos *repositories.Repositories) *MovieUseCases {
	return &MovieUseCases{
		List:    movies.NewListMoviesUseCase(repos.Movie),
		Get:     movies.NewGetMovieUseCase(repos.Movie),
		Search:  movies.NewSearchMoviesUseCase(repos.Movie),
		ListIDs: movies.NewListMovieIDsUseCase(repos.Movie),
	}
}

// buildTVUseCases creates TV show use cases
func buildTVUseCases(repos *repositories.Repositories) *TVUseCases {
	return &TVUseCases{
		ListShows:      tv.NewListTVShowsUseCase(repos.TV),
		GetShow:        tv.NewGetTVShowUseCase(repos.TV),
		ListEpisodes:   tv.NewListTVEpisodesUseCase(repos.TV),
		GetEpisode:     tv.NewGetTVEpisodeUseCase(repos.TV),
		SearchEpisodes: tv.NewSearchTVEpisodesUseCase(repos.TV),
		ListShowIDs:    tv.NewListTVShowIDsUseCase(repos.TV),
		GetNextEpisode: tv.NewGetNextEpisodeUseCase(repos.TV, repos.Progress),
	}
}

// buildMusicUseCases creates music use cases
func buildMusicUseCases(repos *repositories.Repositories) *MusicUseCases {
	return &MusicUseCases{
		ListArtists:        music.NewListArtistsUseCase(repos.Music),
		ListAlbumsByArtist: music.NewListAlbumsByArtistIDUseCase(repos.Music),
		ListTracksByAlbum:  music.NewListTracksByAlbumIDUseCase(repos.Music),
		GetTrack:           music.NewGetTrackUseCase(repos.Music),
		SearchTracks:       music.NewSearchTracksUseCase(repos.Music),
		ListArtistIDs:      music.NewListArtistIDsUseCase(repos.Music),
	}
}

// buildImageUseCases creates image use cases
func buildImageUseCases(
	repos *repositories.Repositories,
	svcs *services.Services,
	logger *slog.Logger,
	imageCacheDir string,
) *ImageUseCases {
	return &ImageUseCases{
		Get:            images.NewGetImageUseCase(repos.Image),
		GetMedia:       images.NewGetMediaImagesUseCase(repos.Image),
		GetEntity:      images.NewGetEntityImagesUseCase(repos.Image),
		GetBatch:       images.NewGetBatchMediaImagesUseCase(repos.Image),
		Cleanup:        images.NewCleanupUseCase(repos.Image, imageCacheDir, logger),
		ExtractMovie:   images.NewExtractMovieImagesUseCase(repos.Image, svcs.ImageCache, svcs.ImageTransformer),
		ExtractEpisode: images.NewExtractTVEpisodeImagesUseCase(repos.Image, svcs.ImageCache, svcs.ImageTransformer),
		ExtractShow:    images.NewExtractTVShowImagesUseCase(repos.Image, svcs.ImageCache, svcs.ImageTransformer),
		ExtractSeason:  images.NewExtractTVSeasonImagesUseCase(repos.Image, svcs.ImageCache, svcs.ImageTransformer),
		ExtractAlbum:   images.NewExtractMusicAlbumImagesUseCase(repos.Image, svcs.ImageCache, svcs.ImageTransformer),
		ExtractArtist:  images.NewExtractMusicArtistImagesUseCase(repos.Image, svcs.ImageCache, svcs.ImageTransformer),
	}
}

// buildTranscodeUseCases creates transcode use cases
func buildTranscodeUseCases(
	repos *repositories.Repositories,
	svcs *services.Services,
) *TranscodeUseCases {
	if svcs.TranscodeQueue == nil {
		return &TranscodeUseCases{}
	}

	createJob := transcode.NewCreateJobUseCase(repos.Transcode, svcs.TranscodeQueue)
	getStatus := transcode.NewGetJobStatusUseCase(repos.Transcode)
	serveManifest := transcode.NewServeManifestUseCase(repos.Media, svcs.SessionManager)

	return &TranscodeUseCases{
		CreateJob:     createJob,
		GetStatus:     getStatus,
		ServeManifest: serveManifest,
	}
}
