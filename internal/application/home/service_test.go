package home

import (
	"context"
	"errors"
	"testing"

	apptrending "github.com/mantonx/viewra/internal/application/trending"
	"github.com/mantonx/viewra/internal/domain/home"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/registry"
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// mockPreferencesRepository implements home.PreferencesRepository for testing.
type mockPreferencesRepository struct {
	prefs     []*home.WidgetPreference
	getErr    error
	saveErr   error
	deleteErr error
}

func (m *mockPreferencesRepository) Get(ctx context.Context, userID string) ([]*home.WidgetPreference, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.prefs, nil
}

func (m *mockPreferencesRepository) GetForWidget(ctx context.Context, userID, widgetID string) (*home.WidgetPreference, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, p := range m.prefs {
		if p.WidgetID == widgetID && p.UserID == userID {
			return p, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockPreferencesRepository) Save(ctx context.Context, pref *home.WidgetPreference) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.prefs = append(m.prefs, pref)
	return nil
}

func (m *mockPreferencesRepository) SaveAll(ctx context.Context, prefs []*home.WidgetPreference) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.prefs = prefs
	return nil
}

func (m *mockPreferencesRepository) Delete(ctx context.Context, userID, widgetID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return nil
}

func (m *mockPreferencesRepository) DeleteAll(ctx context.Context, userID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.prefs = nil
	return nil
}

// mockWidgetDataFetcher implements WidgetDataFetcher for testing.
type mockWidgetDataFetcher struct {
	data map[string]any
	err  error
}

func (m *mockWidgetDataFetcher) FetchWidgetData(ctx context.Context, pluginID, path, userID, clientType string, params map[string]string) (map[string]any, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.data, nil
}

// mockContinueWatchingService implements ContinueWatchingService for testing.
type mockContinueWatchingService struct {
	hasHistory bool
	items      []MediaItemWithTime
	err        error
}

func (m *mockContinueWatchingService) HasHistory(ctx context.Context, userID string) bool {
	return m.hasHistory
}

func (m *mockContinueWatchingService) GetContinueWatchingFull(ctx context.Context, userID string, limit int) ([]MediaItemWithTime, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.items) > limit {
		return m.items[:limit], nil
	}
	return m.items, nil
}

// mockRecentlyAddedService implements RecentlyAddedService for testing.
type mockRecentlyAddedService struct {
	items    []*home.MediaItem
	fullItems []MediaItemWithTime
	err       error
}

func (m *mockRecentlyAddedService) GetRecentlyAdded(ctx context.Context, limit int) ([]*home.MediaItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.items) > limit {
		return m.items[:limit], nil
	}
	return m.items, nil
}

func (m *mockRecentlyAddedService) GetRecentlyAddedFull(ctx context.Context, limit int) ([]MediaItemWithTime, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.fullItems) > limit {
		return m.fullItems[:limit], nil
	}
	return m.fullItems, nil
}

// mockFavoritesService implements FavoritesService for testing.
type mockFavoritesService struct {
	hasFavorites bool
	items        []*home.MediaItem
	fullItems    []MediaItemWithTime
	err          error
}

func (m *mockFavoritesService) HasFavorites(ctx context.Context, userID string) bool {
	return m.hasFavorites
}

func (m *mockFavoritesService) GetFavorites(ctx context.Context, userID string, limit int) ([]*home.MediaItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.items) > limit {
		return m.items[:limit], nil
	}
	return m.items, nil
}

func (m *mockFavoritesService) GetFavoritesFull(ctx context.Context, userID string, limit int) ([]MediaItemWithTime, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.fullItems) > limit {
		return m.fullItems[:limit], nil
	}
	return m.fullItems, nil
}

// mockGenresService implements GenresService for testing.
type mockGenresService struct {
	genres []string
	err    error
}

func (m *mockGenresService) GetDistinctGenres(ctx context.Context, limit int) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.genres) > limit {
		return m.genres[:limit], nil
	}
	return m.genres, nil
}

// mockTrendingService implements TrendingService for testing.
type mockTrendingService struct {
	hasProvider bool
	result      *apptrending.Result
	err         error
}

func (m *mockTrendingService) HasProvider() bool {
	return m.hasProvider
}

func (m *mockTrendingService) GetTrending(ctx context.Context, mediaType string, limit int) (*apptrending.Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// nullLogger returns a nil logger for tests
func nullLogger() *testing.T {
	return nil
}

func TestMatchesClientType(t *testing.T) {
	tests := []struct {
		name        string
		clientTypes []string
		clientType  string
		expected    bool
	}{
		{"empty client types matches all", []string{}, "web", true},
		{"nil client types matches all", nil, "ios", true},
		{"explicit match", []string{"web", "ios"}, "web", true},
		{"no match", []string{"ios", "android"}, "web", false},
		{"all client type matches any", []string{sdk.ClientTypeAll}, "roku", true},
		{"mixed with all", []string{"web", sdk.ClientTypeAll}, "firetv", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesClientType(tt.clientTypes, tt.clientType)
			if result != tt.expected {
				t.Errorf("matchesClientType(%v, %q) = %v, want %v", tt.clientTypes, tt.clientType, result, tt.expected)
			}
		})
	}
}

func TestGetTimeOfDay(t *testing.T) {
	// We can't easily mock time.Now(), but we can verify the function returns valid values
	result := getTimeOfDay()
	validValues := map[string]bool{"morning": true, "afternoon": true, "evening": true, "night": true}
	if !validValues[result] {
		t.Errorf("getTimeOfDay() = %q, want one of morning/afternoon/evening/night", result)
	}
}

func TestGetGreeting(t *testing.T) {
	result := getGreeting()
	validPrefixes := []string{"Good morning", "Good afternoon", "Good evening", "Good night"}
	valid := false
	for _, prefix := range validPrefixes {
		if result == prefix {
			valid = true
			break
		}
	}
	if !valid {
		t.Errorf("getGreeting() = %q, want one of Good morning/afternoon/evening/night", result)
	}
}

func TestGetSeason(t *testing.T) {
	result := getSeason()
	validValues := map[string]bool{"spring": true, "summer": true, "fall": true, "winter": true}
	if !validValues[result] {
		t.Errorf("getSeason() = %q, want one of spring/summer/fall/winter", result)
	}
}

func TestServiceFilterByClientType(t *testing.T) {
	s := &Service{}

	widgets := []*registry.RegisteredWidget{
		{Widget: sdk.Widget{ID: "w1", ClientTypes: []string{"web"}}},
		{Widget: sdk.Widget{ID: "w2", ClientTypes: []string{"ios", "android"}}},
		{Widget: sdk.Widget{ID: "w3", ClientTypes: []string{sdk.ClientTypeAll}}},
		{Widget: sdk.Widget{ID: "w4", ClientTypes: nil}},
	}

	tests := []struct {
		name       string
		clientType string
		wantIDs    []string
	}{
		{"web client", "web", []string{"w1", "w3", "w4"}},
		{"ios client", "ios", []string{"w2", "w3", "w4"}},
		{"android client", "android", []string{"w2", "w3", "w4"}},
		{"roku client", "roku", []string{"w3", "w4"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.filterByClientType(widgets, tt.clientType)
			if len(result) != len(tt.wantIDs) {
				t.Errorf("filterByClientType() returned %d widgets, want %d", len(result), len(tt.wantIDs))
				return
			}
			for i, w := range result {
				if w.Widget.ID != tt.wantIDs[i] {
					t.Errorf("filterByClientType()[%d].ID = %q, want %q", i, w.Widget.ID, tt.wantIDs[i])
				}
			}
		})
	}
}

func TestServiceFilterBySections(t *testing.T) {
	s := &Service{}

	widgets := []*registry.RegisteredWidget{
		{Widget: sdk.Widget{ID: "continue-watching"}},
		{Widget: sdk.Widget{ID: "recently-added"}},
		{Widget: sdk.Widget{ID: "favorites"}},
		{Widget: sdk.Widget{ID: "trending"}},
	}

	tests := []struct {
		name       string
		sectionIDs []string
		wantIDs    []string
	}{
		{"single section", []string{"continue-watching"}, []string{"continue-watching"}},
		{"multiple sections", []string{"favorites", "trending"}, []string{"favorites", "trending"}},
		{"non-existent section", []string{"non-existent"}, []string{}},
		{"mixed existing and non-existing", []string{"recently-added", "foo"}, []string{"recently-added"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.filterBySections(widgets, tt.sectionIDs)
			if len(result) != len(tt.wantIDs) {
				t.Errorf("filterBySections() returned %d widgets, want %d", len(result), len(tt.wantIDs))
				return
			}
			resultIDs := make(map[string]bool)
			for _, w := range result {
				resultIDs[w.Widget.ID] = true
			}
			for _, wantID := range tt.wantIDs {
				if !resultIDs[wantID] {
					t.Errorf("filterBySections() missing widget %q", wantID)
				}
			}
		})
	}
}

func TestServiceApplyPreferences(t *testing.T) {
	s := &Service{}

	widgets := []*registry.RegisteredWidget{
		{Widget: sdk.Widget{ID: "w1", Priority: 100}},
		{Widget: sdk.Widget{ID: "w2", Priority: 50}},
		{Widget: sdk.Widget{ID: "w3", Priority: 75}},
	}

	t.Run("no preferences returns original order unchanged", func(t *testing.T) {
		result := s.applyPreferences(widgets, nil)
		if len(result) != 3 {
			t.Fatalf("expected 3 widgets, got %d", len(result))
		}
		// With no preferences, returns original input order unchanged
		expectedOrder := []string{"w1", "w2", "w3"}
		for i, w := range result {
			if w.Widget.ID != expectedOrder[i] {
				t.Errorf("widget %d: got %q, want %q", i, w.Widget.ID, expectedOrder[i])
			}
		}
	})

	t.Run("preferences override order", func(t *testing.T) {
		prefs := []*home.WidgetPreference{
			{WidgetID: "w2", Position: 0, Hidden: false},
			{WidgetID: "w3", Position: 1, Hidden: false},
		}
		result := s.applyPreferences(widgets, prefs)
		if len(result) != 3 {
			t.Fatalf("expected 3 widgets, got %d", len(result))
		}
		// Preferenced widgets first by position, then non-preferenced by priority
		// w2(pos 0), w3(pos 1), w1(no pref, priority 100)
		expectedOrder := []string{"w2", "w3", "w1"}
		for i, w := range result {
			if w.Widget.ID != expectedOrder[i] {
				t.Errorf("widget %d: got %q, want %q", i, w.Widget.ID, expectedOrder[i])
			}
		}
	})

	t.Run("hidden widgets are excluded", func(t *testing.T) {
		prefs := []*home.WidgetPreference{
			{WidgetID: "w1", Position: 0, Hidden: true},
		}
		result := s.applyPreferences(widgets, prefs)
		if len(result) != 2 {
			t.Fatalf("expected 2 widgets (w1 hidden), got %d", len(result))
		}
		for _, w := range result {
			if w.Widget.ID == "w1" {
				t.Error("hidden widget w1 should not be in result")
			}
		}
	})
}

func TestServiceBuildSectionsWithURLs(t *testing.T) {
	s := &Service{}

	widgets := []*registry.RegisteredWidget{
		{
			Widget:   sdk.Widget{ID: "section1", Type: "media-row", Location: "homepage-sections", Priority: 100, CacheTTLSeconds: 300},
			PluginID: "plugin1",
		},
		{
			Widget:   sdk.Widget{ID: "section2", Type: "featured-row", Location: "homepage-top", Priority: 200},
			PluginID: "plugin2",
		},
	}

	sections := s.buildSectionsWithURLs(widgets)

	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}

	// Check first section
	if sections[0].ID != "section1" {
		t.Errorf("section[0].ID = %q, want %q", sections[0].ID, "section1")
	}
	if sections[0].DataURL != "/api/home/sections/section1" {
		t.Errorf("section[0].DataURL = %q, want %q", sections[0].DataURL, "/api/home/sections/section1")
	}
	if sections[0].Position != 0 {
		t.Errorf("section[0].Position = %d, want 0", sections[0].Position)
	}
	if sections[0].PluginID != "plugin1" {
		t.Errorf("section[0].PluginID = %q, want %q", sections[0].PluginID, "plugin1")
	}

	// Check second section
	if sections[1].ID != "section2" {
		t.Errorf("section[1].ID = %q, want %q", sections[1].ID, "section2")
	}
	if sections[1].Position != 1 {
		t.Errorf("section[1].Position = %d, want 1", sections[1].Position)
	}
}

func TestServiceGetContinueWatchingData(t *testing.T) {
	t.Run("nil service returns empty data", func(t *testing.T) {
		s := &Service{continueWatching: nil}
		data, err := s.getContinueWatchingData(context.Background(), "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data["title"] != "Continue Watching" {
			t.Errorf("title = %v, want %q", data["title"], "Continue Watching")
		}
		if items, ok := data["items"].([]any); !ok || len(items) != 0 {
			t.Error("expected empty items array")
		}
	})

	t.Run("error from service propagates", func(t *testing.T) {
		s := &Service{
			continueWatching: &mockContinueWatchingService{
				err: errors.New("service error"),
			},
		}
		_, err := s.getContinueWatchingData(context.Background(), "user1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestServiceGetRecentlyAddedData(t *testing.T) {
	t.Run("nil service returns empty data", func(t *testing.T) {
		s := &Service{recentlyAdded: nil}
		data, err := s.getRecentlyAddedData(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data["title"] != "Recently Added" {
			t.Errorf("title = %v, want %q", data["title"], "Recently Added")
		}
	})

	t.Run("error from service propagates", func(t *testing.T) {
		s := &Service{
			recentlyAdded: &mockRecentlyAddedService{
				err: errors.New("service error"),
			},
		}
		_, err := s.getRecentlyAddedData(context.Background())
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestServiceGetFavoritesData(t *testing.T) {
	t.Run("nil service returns empty data", func(t *testing.T) {
		s := &Service{favorites: nil}
		data, err := s.getFavoritesData(context.Background(), "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data["title"] != "Your Favorites" {
			t.Errorf("title = %v, want %q", data["title"], "Your Favorites")
		}
	})

	t.Run("error from service propagates", func(t *testing.T) {
		s := &Service{
			favorites: &mockFavoritesService{
				err: errors.New("service error"),
			},
		}
		_, err := s.getFavoritesData(context.Background(), "user1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestServiceGetTrendingData(t *testing.T) {
	t.Run("nil service returns empty data", func(t *testing.T) {
		s := &Service{trending: nil}
		data, err := s.getTrendingData(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data["title"] != "Trending" {
			t.Errorf("title = %v, want %q", data["title"], "Trending")
		}
	})

	t.Run("no provider returns empty data", func(t *testing.T) {
		s := &Service{
			trending: &mockTrendingService{hasProvider: false},
		}
		data, err := s.getTrendingData(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data["title"] != "Trending" {
			t.Errorf("title = %v, want %q", data["title"], "Trending")
		}
	})
}

func TestServiceGetSearchHeroFallbackData(t *testing.T) {
	t.Run("nil genres service", func(t *testing.T) {
		s := &Service{genres: nil}
		data, err := s.getSearchHeroFallbackData(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data["placeholder"] != "Search..." {
			t.Errorf("placeholder = %v, want %q", data["placeholder"], "Search...")
		}
		if data["search_url"] != "/api/search" {
			t.Errorf("search_url = %v, want %q", data["search_url"], "/api/search")
		}
	})

	t.Run("with genres", func(t *testing.T) {
		s := &Service{
			genres: &mockGenresService{
				genres: []string{"Action", "Comedy", "Drama"},
			},
		}
		data, err := s.getSearchHeroFallbackData(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		suggestions, ok := data["suggestions"].([]map[string]any)
		if !ok {
			t.Fatal("suggestions should be []map[string]any")
		}
		if len(suggestions) != 3 {
			t.Errorf("expected 3 suggestions, got %d", len(suggestions))
		}
	})
}

func TestServiceGetPreferences(t *testing.T) {
	prefs := []*home.WidgetPreference{
		{UserID: "user1", WidgetID: "w1", Position: 0},
		{UserID: "user1", WidgetID: "w2", Position: 1},
	}

	s := &Service{
		preferencesRepo: &mockPreferencesRepository{prefs: prefs},
	}

	result, err := s.GetPreferences(context.Background(), "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 preferences, got %d", len(result))
	}
}

func TestServiceResetPreferences(t *testing.T) {
	repo := &mockPreferencesRepository{
		prefs: []*home.WidgetPreference{{WidgetID: "w1"}},
	}

	s := &Service{preferencesRepo: repo}

	err := s.ResetPreferences(context.Background(), "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.prefs != nil {
		t.Error("expected prefs to be nil after reset")
	}
}

func TestServiceBuildMeta(t *testing.T) {
	s := &Service{
		continueWatching: &mockContinueWatchingService{hasHistory: true},
		favorites:        &mockFavoritesService{hasFavorites: true},
		trending:         &mockTrendingService{hasProvider: false},
	}

	meta := s.buildMeta(context.Background(), "user1")

	if meta == nil {
		t.Fatal("expected meta, got nil")
	}
	if meta.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should not be zero")
	}
	if meta.UserContext == nil {
		t.Fatal("expected UserContext, got nil")
	}
	if !meta.UserContext.HasWatchHistory {
		t.Error("HasWatchHistory should be true")
	}
	if !meta.UserContext.HasRatings {
		t.Error("HasRatings should be true")
	}
	if meta.Hero == nil {
		t.Fatal("expected Hero, got nil")
	}
	if meta.Hero.Greeting == "" {
		t.Error("Greeting should not be empty")
	}
}

func TestNewService(t *testing.T) {
	widgetRegistry := registry.NewWidgetRegistry()
	searchProviders := registry.NewSearchProviderRegistry()
	trendingProviders := registry.NewTrendingProviderRegistry()
	capabilityRegistry := registry.NewCapabilityRegistry()
	prefsRepo := &mockPreferencesRepository{}
	fetcher := &mockWidgetDataFetcher{}
	cw := &mockContinueWatchingService{}
	ra := &mockRecentlyAddedService{}
	fav := &mockFavoritesService{}
	genres := &mockGenresService{}
	trending := &mockTrendingService{}

	s := NewService(
		widgetRegistry,
		searchProviders,
		trendingProviders,
		capabilityRegistry,
		prefsRepo,
		fetcher,
		cw,
		ra,
		fav,
		genres,
		trending,
		nil, // logger
	)

	if s == nil {
		t.Fatal("NewService returned nil")
	}
	if s.widgetRegistry != widgetRegistry {
		t.Error("widgetRegistry not set correctly")
	}
	// Skip direct pointer comparison for interface types
	if s.preferencesRepo == nil {
		t.Error("preferencesRepo should not be nil")
	}
}
