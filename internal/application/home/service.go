package home

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	apptrending "github.com/mantonx/viewra/internal/application/trending"
	"github.com/mantonx/viewra/internal/domain/home"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/registry"
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// WidgetDataFetcher fetches data for a widget from a plugin.
type WidgetDataFetcher interface {
	// FetchWidgetData fetches data for a widget.
	// The path is the widget's configured endpoint (e.g., "/recommendations").
	// The params map contains additional query parameters from endpoint_params config.
	FetchWidgetData(ctx context.Context, pluginID, path, userID, clientType string, params map[string]string) (map[string]any, error)
}

// ContinueWatchingService provides continue watching data.
type ContinueWatchingService interface {
	// HasHistory returns true if the user has watch history.
	HasHistory(ctx context.Context, userID string) bool

	// GetContinueWatchingFull returns items the user is currently watching with full typed data.
	GetContinueWatchingFull(ctx context.Context, userID string, limit int) ([]MediaItemWithTime, error)
}

// RecentlyAddedService provides recently added media data.
type RecentlyAddedService interface {
	// GetRecentlyAdded returns recently added media items.
	GetRecentlyAdded(ctx context.Context, limit int) ([]*home.MediaItem, error)

	// GetRecentlyAddedFull returns recently added media with full typed data.
	GetRecentlyAddedFull(ctx context.Context, limit int) ([]MediaItemWithTime, error)
}

// FavoritesService provides user favorites data.
type FavoritesService interface {
	// HasFavorites returns true if the user has any favorites.
	HasFavorites(ctx context.Context, userID string) bool

	// GetFavorites returns the user's favorite media items.
	GetFavorites(ctx context.Context, userID string, limit int) ([]*home.MediaItem, error)

	// GetFavoritesFull returns favorite media with full typed data.
	GetFavoritesFull(ctx context.Context, userID string, limit int) ([]MediaItemWithTime, error)
}

// GenresService provides genre data for search suggestions.
type GenresService interface {
	// GetDistinctGenres returns distinct genres from the library.
	GetDistinctGenres(ctx context.Context, limit int) ([]string, error)
}

// TrendingService provides trending media data.
type TrendingService interface {
	// GetTrending returns trending items matched to the local library.
	GetTrending(ctx context.Context, mediaType string, limit int) (*apptrending.Result, error)
	// HasProvider returns true if a trending provider is available.
	HasProvider() bool
}

// Service aggregates widgets from all plugins and builds the home response.
type Service struct {
	widgetRegistry     *registry.WidgetRegistry
	searchProviders    *registry.SearchProviderRegistry
	trendingProviders  *registry.TrendingProviderRegistry
	capabilityRegistry *registry.CapabilityRegistry
	preferencesRepo    home.PreferencesRepository
	widgetDataFetcher  WidgetDataFetcher
	continueWatching   ContinueWatchingService
	recentlyAdded      RecentlyAddedService
	favorites          FavoritesService
	genres             GenresService
	trending           TrendingService
	logger             *slog.Logger
}

// NewService creates a new home service.
func NewService(
	widgetRegistry *registry.WidgetRegistry,
	searchProviders *registry.SearchProviderRegistry,
	trendingProviders *registry.TrendingProviderRegistry,
	capabilityRegistry *registry.CapabilityRegistry,
	preferencesRepo home.PreferencesRepository,
	widgetDataFetcher WidgetDataFetcher,
	continueWatching ContinueWatchingService,
	recentlyAdded RecentlyAddedService,
	favorites FavoritesService,
	genres GenresService,
	trending TrendingService,
	logger *slog.Logger,
) *Service {
	return &Service{
		widgetRegistry:     widgetRegistry,
		searchProviders:    searchProviders,
		trendingProviders:  trendingProviders,
		capabilityRegistry: capabilityRegistry,
		preferencesRepo:    preferencesRepo,
		widgetDataFetcher:  widgetDataFetcher,
		continueWatching:   continueWatching,
		recentlyAdded:      recentlyAdded,
		favorites:          favorites,
		genres:             genres,
		trending:           trending,
		logger:             logger,
	}
}

// GetHome returns the home screen data.
func (s *Service) GetHome(ctx context.Context, req *home.HomeRequest) (*home.HomeResponse, error) {
	// 1. Get all enabled widgets
	widgets := s.widgetRegistry.GetEnabled()

	// 2. Filter by client type
	if req.ClientType != "" {
		widgets = s.filterByClientType(widgets, req.ClientType)
	}

	// 3. Filter by required capabilities
	widgets = s.filterByCapabilities(widgets)

	// 4. Get user preferences
	prefs, err := s.preferencesRepo.Get(ctx, req.UserID)
	if err != nil {
		s.logger.Warn("failed to get user preferences", "error", err, "user_id", req.UserID)
		// Continue without preferences
	}

	// 5. Apply preferences (ordering, visibility)
	widgets = s.applyPreferences(widgets, prefs)

	// 6. If no preferences, apply smart defaults
	if len(prefs) == 0 {
		widgets = s.applySmartDefaults(ctx, req.UserID, widgets)
	}

	// 7. Filter by requested sections if specified
	if len(req.Sections) > 0 {
		widgets = s.filterBySections(widgets, req.Sections)
	}

	// 8. Fetch data for each widget (parallel)
	var sections []*home.Section
	if req.Inline {
		sections = s.fetchWidgetData(ctx, widgets, req)
	} else {
		sections = s.buildSectionsWithURLs(widgets)
	}

	// 9. Build response
	return &home.HomeResponse{
		Sections: sections,
		Preferences: &home.PreferencesInfo{
			CanReorder: true,
			CanHide:    true,
			UpdateURL:  "/api/home/preferences",
		},
		Meta: s.buildMeta(ctx, req.UserID),
	}, nil
}

// GetSection returns a single section's data.
func (s *Service) GetSection(ctx context.Context, sectionID, userID, clientType string) (*home.Section, error) {
	widget := s.widgetRegistry.GetByID(sectionID)
	if widget == nil {
		return nil, fmt.Errorf("section not found: %s", sectionID)
	}

	// Check capability
	if widget.Widget.RequiredCapability != "" {
		if s.capabilityRegistry.Resolve(widget.Widget.RequiredCapability) == nil {
			return nil, fmt.Errorf("required capability not available: %s", widget.Widget.RequiredCapability)
		}
	}

	data, err := s.getWidgetData(ctx, widget, userID, clientType)
	if err != nil {
		return nil, fmt.Errorf("fetch widget data: %w", err)
	}

	return &home.Section{
		ID:              widget.Widget.ID,
		Type:            widget.Widget.Type,
		Location:        widget.Widget.Location,
		ClientTypes:     widget.Widget.ClientTypes,
		Priority:        widget.Widget.Priority,
		CacheTTLSeconds: widget.Widget.CacheTTLSeconds,
		Data:            data,
		PluginID:        widget.PluginID,
	}, nil
}

// GetPreferences returns the user's widget preferences.
func (s *Service) GetPreferences(ctx context.Context, userID string) ([]*home.WidgetPreference, error) {
	return s.preferencesRepo.Get(ctx, userID)
}

// UpdatePreferences updates the user's widget preferences.
func (s *Service) UpdatePreferences(ctx context.Context, userID string, req *home.PreferencesUpdateRequest) error {
	prefs := make([]*home.WidgetPreference, 0, len(req.Sections))
	now := time.Now()

	for _, sp := range req.Sections {
		// Verify the widget exists
		widget := s.widgetRegistry.GetByID(sp.ID)
		if widget == nil {
			continue // Skip unknown widgets
		}

		prefs = append(prefs, &home.WidgetPreference{
			UserID:    userID,
			WidgetID:  sp.ID,
			Location:  widget.Widget.Location,
			Position:  sp.Position,
			Hidden:    sp.Hidden,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	return s.preferencesRepo.SaveAll(ctx, prefs)
}

// ResetPreferences removes all user preferences, resetting to defaults.
func (s *Service) ResetPreferences(ctx context.Context, userID string) error {
	return s.preferencesRepo.DeleteAll(ctx, userID)
}

// filterByClientType filters widgets to those matching the client type.
func (s *Service) filterByClientType(widgets []*registry.RegisteredWidget, clientType string) []*registry.RegisteredWidget {
	result := make([]*registry.RegisteredWidget, 0)
	for _, w := range widgets {
		if matchesClientType(w.Widget.ClientTypes, clientType) {
			result = append(result, w)
		}
	}
	return result
}

// filterByCapabilities removes widgets whose required capabilities aren't available.
func (s *Service) filterByCapabilities(widgets []*registry.RegisteredWidget) []*registry.RegisteredWidget {
	result := make([]*registry.RegisteredWidget, 0)
	for _, w := range widgets {
		if w.Widget.RequiredCapability == "" {
			result = append(result, w)
			continue
		}
		if s.capabilityRegistry.Resolve(w.Widget.RequiredCapability) != nil {
			result = append(result, w)
		}
	}
	return result
}

// filterBySections filters to only the requested section IDs.
func (s *Service) filterBySections(widgets []*registry.RegisteredWidget, sectionIDs []string) []*registry.RegisteredWidget {
	sectionSet := make(map[string]bool)
	for _, id := range sectionIDs {
		sectionSet[id] = true
	}

	result := make([]*registry.RegisteredWidget, 0)
	for _, w := range widgets {
		if sectionSet[w.Widget.ID] {
			result = append(result, w)
		}
	}
	return result
}

// applyPreferences applies user preferences to widgets.
func (s *Service) applyPreferences(widgets []*registry.RegisteredWidget, prefs []*home.WidgetPreference) []*registry.RegisteredWidget {
	if len(prefs) == 0 {
		return widgets
	}

	// Build preference map
	prefMap := make(map[string]*home.WidgetPreference)
	for _, p := range prefs {
		prefMap[p.WidgetID] = p
	}

	// Apply preferences and track positioned widgets
	type widgetWithPosition struct {
		widget   *registry.RegisteredWidget
		position int
		hasPrefs bool
	}

	positioned := make([]widgetWithPosition, 0, len(widgets))
	for _, w := range widgets {
		wp := widgetWithPosition{widget: w, position: w.Widget.Priority}
		if pref, ok := prefMap[w.Widget.ID]; ok {
			if pref.Hidden {
				continue // Skip hidden widgets
			}
			wp.position = pref.Position
			wp.hasPrefs = true
		}
		positioned = append(positioned, wp)
	}

	// Sort: preferenced widgets first by position, then non-preferenced by priority
	sort.Slice(positioned, func(i, j int) bool {
		if positioned[i].hasPrefs && positioned[j].hasPrefs {
			return positioned[i].position < positioned[j].position
		}
		if positioned[i].hasPrefs {
			return true
		}
		if positioned[j].hasPrefs {
			return false
		}
		return positioned[i].widget.Widget.Priority > positioned[j].widget.Widget.Priority
	})

	result := make([]*registry.RegisteredWidget, 0, len(positioned))
	for _, wp := range positioned {
		result = append(result, wp.widget)
	}
	return result
}

// applySmartDefaults applies intelligent default ordering based on user context.
func (s *Service) applySmartDefaults(ctx context.Context, userID string, widgets []*registry.RegisteredWidget) []*registry.RegisteredWidget {
	hasWatchHistory := false
	if s.continueWatching != nil {
		hasWatchHistory = s.continueWatching.HasHistory(ctx, userID)
	}

	// Create a copy to avoid modifying the original
	result := make([]*registry.RegisteredWidget, len(widgets))
	copy(result, widgets)

	// Adjust priorities based on context
	priorities := make(map[string]int)
	for _, w := range result {
		priorities[w.Widget.ID] = w.Widget.Priority

		switch w.Widget.ID {
		case "continue-watching":
			if hasWatchHistory {
				priorities[w.Widget.ID] += 50 // Boost to top
			} else {
				priorities[w.Widget.ID] = -1000 // Hide if empty
			}

		case "rec-for-you":
			if hasWatchHistory {
				priorities[w.Widget.ID] += 20 // Boost if we have data
			}

		case "rec-trending":
			if !hasWatchHistory {
				priorities[w.Widget.ID] += 30 // Boost for new users
			}
		}
	}

	// Sort by adjusted priority
	sort.Slice(result, func(i, j int) bool {
		pi := priorities[result[i].Widget.ID]
		pj := priorities[result[j].Widget.ID]
		return pi > pj
	})

	// Filter out widgets with negative priority (hidden)
	filtered := make([]*registry.RegisteredWidget, 0)
	for _, w := range result {
		if priorities[w.Widget.ID] >= 0 {
			filtered = append(filtered, w)
		}
	}

	return filtered
}

// fetchWidgetData fetches data for all widgets in parallel.
func (s *Service) fetchWidgetData(ctx context.Context, widgets []*registry.RegisteredWidget, req *home.HomeRequest) []*home.Section {
	sections := make([]*home.Section, 0, len(widgets))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, w := range widgets {
		wg.Add(1)
		go func(widget *registry.RegisteredWidget, position int) {
			defer wg.Done()

			data, err := s.getWidgetData(ctx, widget, req.UserID, req.ClientType)
			if err != nil {
				s.logger.Error("widget data fetch failed",
					"widget_id", widget.Widget.ID,
					"plugin_id", widget.PluginID,
					"error", err,
				)
				return // Skip this widget silently
			}

			mu.Lock()
			sections = append(sections, &home.Section{
				ID:              widget.Widget.ID,
				Type:            widget.Widget.Type,
				Location:        widget.Widget.Location,
				ClientTypes:     widget.Widget.ClientTypes,
				Priority:        widget.Widget.Priority,
				Position:        position,
				Hidden:          false,
				CacheTTLSeconds: widget.Widget.CacheTTLSeconds,
				Data:            data,
				DataURL:         fmt.Sprintf("/api/home/sections/%s", widget.Widget.ID),
				PluginID:        widget.PluginID,
			})
			mu.Unlock()
		}(w, i)
	}

	wg.Wait()

	// Sort by position
	sort.Slice(sections, func(i, j int) bool {
		return sections[i].Position < sections[j].Position
	})

	return sections
}

// buildSectionsWithURLs builds section metadata without data (for inline=false).
func (s *Service) buildSectionsWithURLs(widgets []*registry.RegisteredWidget) []*home.Section {
	sections := make([]*home.Section, 0, len(widgets))
	for i, w := range widgets {
		sections = append(sections, &home.Section{
			ID:              w.Widget.ID,
			Type:            w.Widget.Type,
			Location:        w.Widget.Location,
			ClientTypes:     w.Widget.ClientTypes,
			Priority:        w.Widget.Priority,
			Position:        i,
			CacheTTLSeconds: w.Widget.CacheTTLSeconds,
			DataURL:         fmt.Sprintf("/api/home/sections/%s", w.Widget.ID),
			PluginID:        w.PluginID,
		})
	}
	return sections
}

// getWidgetData fetches data for a single widget.
func (s *Service) getWidgetData(ctx context.Context, widget *registry.RegisteredWidget, userID, clientType string) (map[string]any, error) {
	// Get the endpoint from widget config
	endpoint, _ := widget.Widget.Config["endpoint"].(string)
	if endpoint == "" {
		// Try to generate data from built-in handlers
		return s.getBuiltinWidgetData(ctx, widget, userID)
	}

	// Get endpoint_params if configured (for widgets that need query params)
	var endpointParams map[string]string
	if paramsRaw, ok := widget.Widget.Config["endpoint_params"]; ok {
		if paramsMap, ok := paramsRaw.(map[string]any); ok {
			endpointParams = make(map[string]string)
			for k, v := range paramsMap {
				if str, ok := v.(string); ok {
					endpointParams[k] = str
				}
			}
		}
	}

	// Fetch from plugin
	if s.widgetDataFetcher == nil {
		return nil, fmt.Errorf("no widget data fetcher configured")
	}

	return s.widgetDataFetcher.FetchWidgetData(ctx, widget.PluginID, endpoint, userID, clientType, endpointParams)
}

// getBuiltinWidgetData handles built-in widget types.
func (s *Service) getBuiltinWidgetData(ctx context.Context, widget *registry.RegisteredWidget, userID string) (map[string]any, error) {
	// Route by widget ID for core widgets
	switch widget.Widget.ID {
	case "continue-watching":
		return s.getContinueWatchingData(ctx, userID)
	case "recently-added":
		return s.getRecentlyAddedData(ctx)
	case "favorites":
		return s.getFavoritesData(ctx, userID)
	case "trending":
		return s.getTrendingData(ctx)
	case "search-hero-fallback":
		return s.getSearchHeroFallbackData(ctx)
	default:
		// Fallback for unknown widgets
		return map[string]any{
			"title": widget.Widget.Config["title"],
		}, nil
	}
}

// getContinueWatchingData gets continue watching data with full typed objects.
// Returns data in two formats:
// - "items": array of ContinueWatchingItem for the new horizontal card layout
// - "movies"/"shows": legacy format for backward compatibility
func (s *Service) getContinueWatchingData(ctx context.Context, userID string) (map[string]any, error) {
	if s.continueWatching == nil {
		return map[string]any{
			"title":  "Continue Watching",
			"items":  []any{},
			"movies": []any{},
			"shows":  []any{},
		}, nil
	}

	mediaItems, err := s.continueWatching.GetContinueWatchingFull(ctx, userID, 20)
	if err != nil {
		return nil, err
	}

	// Build the new items array with ContinueWatchingItem format
	items := make([]*home.ContinueWatchingItem, 0, len(mediaItems))
	// Also keep legacy format for backward compatibility
	movieItems := make([]any, 0)
	showItems := make([]any, 0)

	for _, item := range mediaItems {
		if item.Type == "movie" && item.Movie != nil {
			// Add to legacy format
			movieItems = append(movieItems, item.Movie)

			// Build backdrop URL
			backdropURL := ""
			if item.Movie.ID > 0 {
				backdropURL = fmt.Sprintf("/api/movies/%d/images/fanart", item.Movie.ID)
			}

			// Add to new items format
			items = append(items, &home.ContinueWatchingItem{
				EntityType:     "movie",
				EntityID:       int64(item.Movie.ID),
				Title:          item.Movie.Title,
				Year:           item.Movie.Year,
				BackdropURL:    backdropURL,
				Progress:       item.Progress,
				EpisodeContext: nil,
				LastWatchedAt:  item.CreatedAt,
			})
		} else if item.Type == "tv_show" && item.TVShow != nil {
			// Add to legacy format
			showItems = append(showItems, item.TVShow)

			// Build backdrop URL
			backdropURL := ""
			if item.TVShow.ID > 0 {
				backdropURL = fmt.Sprintf("/api/tv/shows/%d/images/fanart", item.TVShow.ID)
			}

			// Add to new items format
			items = append(items, &home.ContinueWatchingItem{
				EntityType:     "tv_show",
				EntityID:       item.TVShow.ID,
				Title:          item.TVShow.Title,
				Year:           item.TVShow.Year,
				BackdropURL:    backdropURL,
				Progress:       item.Progress,
				EpisodeContext: item.EpisodeContext,
				LastWatchedAt:  item.CreatedAt,
			})
		}
	}

	return map[string]any{
		"title":  "Continue Watching",
		"items":  items,
		"movies": movieItems,
		"shows":  showItems,
	}, nil
}

// getRecentlyAddedData gets recently added media data with full typed objects.
func (s *Service) getRecentlyAddedData(ctx context.Context) (map[string]any, error) {
	if s.recentlyAdded == nil {
		return map[string]any{"title": "Recently Added", "movies": []any{}, "shows": []any{}}, nil
	}

	items, err := s.recentlyAdded.GetRecentlyAddedFull(ctx, 20)
	if err != nil {
		return nil, err
	}

	// Separate into typed arrays for frontend
	movieItems := make([]any, 0)
	showItems := make([]any, 0)
	for _, item := range items {
		if item.Type == "movie" && item.Movie != nil {
			movieItems = append(movieItems, item.Movie)
		} else if item.Type == "tv_show" && item.TVShow != nil {
			showItems = append(showItems, item.TVShow)
		}
	}

	return map[string]any{
		"title":  "Recently Added",
		"movies": movieItems,
		"shows":  showItems,
	}, nil
}

// getFavoritesData gets user favorites data with full typed objects.
func (s *Service) getFavoritesData(ctx context.Context, userID string) (map[string]any, error) {
	if s.favorites == nil {
		return map[string]any{"title": "Your Favorites", "movies": []any{}, "shows": []any{}}, nil
	}

	items, err := s.favorites.GetFavoritesFull(ctx, userID, 20)
	if err != nil {
		return nil, err
	}

	// Separate into typed arrays for frontend
	movieItems := make([]any, 0)
	showItems := make([]any, 0)
	for _, item := range items {
		if item.Type == "movie" && item.Movie != nil {
			movieItems = append(movieItems, item.Movie)
		} else if item.Type == "tv_show" && item.TVShow != nil {
			showItems = append(showItems, item.TVShow)
		}
	}

	return map[string]any{
		"title":  "Your Favorites",
		"movies": movieItems,
		"shows":  showItems,
	}, nil
}

// getTrendingData gets trending media data via the trending service.
func (s *Service) getTrendingData(ctx context.Context) (map[string]any, error) {
	if s.trending == nil || !s.trending.HasProvider() {
		return map[string]any{
			"title": "Trending",
			"items": []any{},
		}, nil
	}

	result, err := s.trending.GetTrending(ctx, "all", 20)
	if err != nil {
		s.logger.Warn("failed to get trending data", "error", err)
		return map[string]any{
			"title": "Trending",
			"items": []any{},
		}, nil
	}

	return map[string]any{
		"title":          "Trending",
		"items":          result.Items,
		"source":         result.Source,
		"window":         result.Window,
		"total_matched":  result.TotalMatched,
		"total_trending": result.TotalTrending,
	}, nil
}

// getSearchHeroFallbackData gets search hero data with genre suggestions.
func (s *Service) getSearchHeroFallbackData(ctx context.Context) (map[string]any, error) {
	suggestions := make([]map[string]any, 0)

	if s.genres != nil {
		genres, err := s.genres.GetDistinctGenres(ctx, 8)
		if err == nil {
			for _, g := range genres {
				suggestions = append(suggestions, map[string]any{
					"id":    g,
					"label": g,
					"action": map[string]any{
						"type":   "filter",
						"filter": map[string]string{"genre": g},
					},
				})
			}
		}
	}

	return map[string]any{
		"placeholder": "Search...",
		"suggestions": suggestions,
		"search_url":  "/api/search",
	}, nil
}

// buildMeta builds the response metadata.
func (s *Service) buildMeta(ctx context.Context, userID string) *home.HomeMeta {
	hasWatchHistory := false
	if s.continueWatching != nil {
		hasWatchHistory = s.continueWatching.HasHistory(ctx, userID)
	}

	hasRatings := false
	if s.favorites != nil {
		hasRatings = s.favorites.HasFavorites(ctx, userID)
	}

	return &home.HomeMeta{
		GeneratedAt: time.Now(),
		UserContext: &home.UserContext{
			HasWatchHistory: hasWatchHistory,
			HasRatings:      hasRatings,
			TimeOfDay:       getTimeOfDay(),
			Season:          getSeason(),
		},
		Hero: s.buildHeroData(ctx, userID),
	}
}

// buildHeroData builds the hero backdrop data.
// Sources backdrop from continue watching (first item) or trending.
func (s *Service) buildHeroData(ctx context.Context, userID string) *home.HeroData {
	hero := &home.HeroData{
		Greeting: getGreeting(),
		DateText: time.Now().Format("Monday, January 2"),
	}

	// Try to get backdrop from continue watching (most relevant to user)
	if s.continueWatching != nil {
		items, err := s.continueWatching.GetContinueWatchingFull(ctx, userID, 1)
		if err == nil && len(items) > 0 {
			item := items[0]
			if item.Type == "movie" && item.Movie != nil {
				hero.BackdropMediaID = int64(item.Movie.ID)
				hero.BackdropMediaType = "movie"
				return hero
			} else if item.Type == "tv_show" && item.TVShow != nil {
				hero.BackdropMediaID = item.TVShow.ID
				hero.BackdropMediaType = "tv_show"
				return hero
			}
		}
	}

	// Fallback to trending if no continue watching
	if s.trending != nil && s.trending.HasProvider() {
		result, err := s.trending.GetTrending(ctx, "all", 1)
		if err == nil && len(result.Items) > 0 && result.Items[0].LocalID != nil {
			hero.BackdropMediaID = int64(*result.Items[0].LocalID)
			hero.BackdropMediaType = result.Items[0].MediaType
			return hero
		}
	}

	// No backdrop available
	return hero
}

// getGreeting returns a time-based greeting.
func getGreeting() string {
	hour := time.Now().Hour()
	switch {
	case hour >= 5 && hour < 12:
		return "Good morning"
	case hour >= 12 && hour < 17:
		return "Good afternoon"
	case hour >= 17 && hour < 21:
		return "Good evening"
	default:
		return "Good night"
	}
}

// matchesClientType checks if a widget's client types include the given type.
func matchesClientType(widgetClientTypes []string, clientType string) bool {
	if len(widgetClientTypes) == 0 {
		return true
	}
	for _, ct := range widgetClientTypes {
		if ct == sdk.ClientTypeAll || ct == clientType {
			return true
		}
	}
	return false
}

// getTimeOfDay returns the current time of day.
func getTimeOfDay() string {
	hour := time.Now().Hour()
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 21:
		return "evening"
	default:
		return "night"
	}
}

// getSeason returns the current season (Northern Hemisphere).
func getSeason() string {
	month := time.Now().Month()
	switch {
	case month >= 3 && month <= 5:
		return "spring"
	case month >= 6 && month <= 8:
		return "summer"
	case month >= 9 && month <= 11:
		return "fall"
	default:
		return "winter"
	}
}
