package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// MoodTagService generates mood/vibe tags for media using LLM.
// Uses sdk.AIClient to invoke chat methods on a provider.
type MoodTagService struct {
	ai     *sdk.AIClient
	data   *sdk.DataClient
	logger *slog.Logger

	// Generation state
	mu           sync.RWMutex
	isGenerating bool
	cancelFunc   context.CancelFunc
	progress     MoodTagProgress
}

// MoodTagProgress tracks mood tag generation progress.
type MoodTagProgress struct {
	EntityType  EntityType `json:"entity_type"`
	Total       int64      `json:"total"`
	Processed   int64      `json:"processed"`
	Failed      int64      `json:"failed"`
	LastError   string     `json:"last_error,omitempty"`
	StartedAt   int64      `json:"started_at"`
	LastUpdated int64      `json:"last_updated"`
}

// MoodTags represents generated mood tags for a media item.
type MoodTags struct {
	EntityType  EntityType `json:"entity_type"`
	EntityID    int64      `json:"entity_id"`
	Tags        []string   `json:"tags"`
	GeneratedAt time.Time  `json:"generated_at"`
}

// NewMoodTagService creates a new mood tag service.
func NewMoodTagService(
	plugins *sdk.PluginsClient,
	data *sdk.DataClient,
	logger *slog.Logger,
) *MoodTagService {
	return &MoodTagService{
		ai:     sdk.NewAIClient(plugins),
		data:   data,
		logger: logger,
	}
}

// GenerateForMedia generates mood tags for a single media item.
func (s *MoodTagService) GenerateForMedia(
	ctx context.Context,
	details *sdk.MediaDetails,
) ([]string, error) {
	if s.ai == nil {
		return nil, fmt.Errorf("AI client not available")
	}

	prompt := s.buildPrompt(details)
	if prompt == "" {
		return nil, fmt.Errorf("insufficient metadata for mood tag generation")
	}

	s.logger.Debug("calling chat provider for mood tags",
		"entity_id", details.ID,
		"media_type", details.MediaType,
		"prompt_length", len(prompt),
	)

	// Use AIClient for chat
	resp, err := s.ai.Chat(ctx, sdk.ChatRequest{
		Messages: []sdk.ChatMessage{
			{Role: "system", Content: moodTagSystemPrompt},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.3, // Low temperature for consistent results
		MaxTokens:   100, // Tags should be short
	})
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}

	tags := s.parseTags(resp.Content)
	if len(tags) == 0 {
		return nil, fmt.Errorf("no tags parsed from response: %s", resp.Content)
	}

	s.logger.Debug("generated mood tags",
		"entity_type", details.MediaType,
		"entity_id", details.ID,
		"tags", tags,
	)

	return tags, nil
}

// GenerateForLibrary generates mood tags for all media in a library.
func (s *MoodTagService) GenerateForLibrary(
	ctx context.Context,
	libraryID int64,
	callback func(entityType EntityType, entityID int64, tags []string) error,
) error {
	s.mu.Lock()
	if s.isGenerating {
		s.mu.Unlock()
		return fmt.Errorf("mood tag generation already in progress")
	}
	s.isGenerating = true
	ctx, s.cancelFunc = context.WithCancel(ctx)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isGenerating = false
		s.cancelFunc = nil
		s.mu.Unlock()
	}()

	s.logger.Info("starting mood tag generation", "library_id", libraryID)

	// Verify data client is available
	if s.data == nil {
		s.logger.Error("data client is nil in mood tag generation")
		return fmt.Errorf("data client is nil")
	}

	// List media with pagination
	var processed, failed int64
	offset := 0
	limit := 100

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("mood tag generation cancelled")
			return ctx.Err()
		default:
		}

		s.logger.Debug("listing media for mood tags",
			"library_id", libraryID,
			"offset", offset,
			"limit", limit,
		)

		mediaList, err := s.data.ListMediaByLibrary(ctx, libraryID, limit, offset)
		if err != nil {
			s.logger.Error("failed to list media for mood tags",
				"library_id", libraryID,
				"error", err,
			)
			return fmt.Errorf("list media: %w", err)
		}

		s.logger.Debug("listed media for mood tags",
			"library_id", libraryID,
			"items", len(mediaList.Items),
			"total", mediaList.Total,
			"has_more", mediaList.HasMore,
		)

		if len(mediaList.Items) == 0 {
			s.logger.Debug("no items returned, ending pagination",
				"library_id", libraryID,
				"offset", offset,
			)
			break // No more items
		}

		total := int64(mediaList.Total)
		s.updateProgress(EntityType(mediaList.Items[0].MediaType), total, processed, failed, "")

		for _, media := range mediaList.Items {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			tags, err := s.GenerateForMedia(ctx, media)
			if err != nil {
				s.logger.Warn("failed to generate mood tags",
					"entity_id", media.ID,
					"media_type", media.MediaType,
					"title", media.Title,
					"error", err,
				)
				atomic.AddInt64(&failed, 1)
				s.updateProgress(EntityType(media.MediaType), total, processed, failed, err.Error())
				continue
			}

			s.logger.Debug("generated mood tags",
				"entity_id", media.ID,
				"tags", tags,
			)

			if callback != nil {
				if err := callback(EntityType(media.MediaType), media.ID, tags); err != nil {
					s.logger.Warn("callback failed",
						"entity_id", media.ID,
						"error", err,
					)
				}
			}

			atomic.AddInt64(&processed, 1)
			if processed%10 == 0 {
				s.updateProgress(EntityType(media.MediaType), total, processed, failed, "")
			}
		}

		if !mediaList.HasMore {
			break
		}
		offset += limit
	}

	s.logger.Info("mood tag generation complete",
		"processed", processed,
		"failed", failed,
	)

	return nil
}

// Cancel cancels any running mood tag generation.
func (s *MoodTagService) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
}

// IsGenerating returns whether mood tag generation is in progress.
func (s *MoodTagService) IsGenerating() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isGenerating
}

// GetProgress returns the current generation progress.
func (s *MoodTagService) GetProgress() *MoodTagProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.isGenerating {
		return nil
	}
	progress := s.progress
	return &progress
}

func (s *MoodTagService) updateProgress(
	entityType EntityType,
	total, processed, failed int64,
	lastError string,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	if s.progress.StartedAt == 0 {
		s.progress.StartedAt = now
	}
	s.progress = MoodTagProgress{
		EntityType:  entityType,
		Total:       total,
		Processed:   processed,
		Failed:      failed,
		LastError:   lastError,
		StartedAt:   s.progress.StartedAt,
		LastUpdated: now,
	}
}

// buildPrompt creates the LLM prompt for mood tag generation.
func (s *MoodTagService) buildPrompt(details *sdk.MediaDetails) string {
	if details == nil {
		return ""
	}

	var b strings.Builder

	switch details.MediaType {
	case "movie":
		b.WriteString(fmt.Sprintf("Movie: %s", details.Title))
		if details.Year > 0 {
			b.WriteString(fmt.Sprintf(" (%d)", details.Year))
		}
		b.WriteString("\n")
		if len(details.Genres) > 0 {
			b.WriteString(fmt.Sprintf("Genres: %s\n", strings.Join(details.Genres, ", ")))
		}
		if details.Plot != "" {
			b.WriteString(fmt.Sprintf("Plot: %s\n", details.Plot))
		}
		if len(details.Directors) > 0 {
			b.WriteString(fmt.Sprintf("Director: %s\n", strings.Join(details.Directors, ", ")))
		}
		if details.Tagline != "" {
			b.WriteString(fmt.Sprintf("Tagline: %s\n", details.Tagline))
		}

	case "tv_show":
		b.WriteString(fmt.Sprintf("TV Show: %s", details.Title))
		if details.Year > 0 {
			b.WriteString(fmt.Sprintf(" (%d)", details.Year))
		}
		b.WriteString("\n")
		if len(details.Genres) > 0 {
			b.WriteString(fmt.Sprintf("Genres: %s\n", strings.Join(details.Genres, ", ")))
		}
		if details.Plot != "" {
			b.WriteString(fmt.Sprintf("Plot: %s\n", details.Plot))
		}
		if details.OriginalLanguage != "" {
			b.WriteString(fmt.Sprintf("Language: %s\n", details.OriginalLanguage))
		}

	case "tv_episode":
		if details.ShowTitle != "" {
			b.WriteString(fmt.Sprintf("TV Episode from: %s\n", details.ShowTitle))
		}
		b.WriteString(fmt.Sprintf("Episode: S%02dE%02d - %s\n",
			details.SeasonNumber, details.EpisodeNumber, details.Title))
		if details.Plot != "" {
			b.WriteString(fmt.Sprintf("Plot: %s\n", details.Plot))
		}

	case "music_album":
		b.WriteString(fmt.Sprintf("Music Album: %s", details.Title))
		if details.ArtistName != "" {
			b.WriteString(fmt.Sprintf(" by %s", details.ArtistName))
		}
		if details.Year > 0 {
			b.WriteString(fmt.Sprintf(" (%d)", details.Year))
		}
		b.WriteString("\n")
		if len(details.Genres) > 0 {
			b.WriteString(fmt.Sprintf("Genres: %s\n", strings.Join(details.Genres, ", ")))
		}
		if details.ReleaseType != "" {
			b.WriteString(fmt.Sprintf("Type: %s\n", details.ReleaseType))
		}
		b.WriteString("(Use MUSIC-SPECIFIC mood tags from the list)\n")

	case "music_artist":
		b.WriteString(fmt.Sprintf("Music Artist: %s\n", details.Title))
		if len(details.Genres) > 0 {
			b.WriteString(fmt.Sprintf("Genres: %s\n", strings.Join(details.Genres, ", ")))
		}
		if details.Biography != "" {
			// Truncate long biographies
			bio := details.Biography
			if len(bio) > 500 {
				bio = bio[:500] + "..."
			}
			b.WriteString(fmt.Sprintf("Bio: %s\n", bio))
		}
		b.WriteString("(Use MUSIC-SPECIFIC mood tags from the list)\n")

	default:
		// For other types, use a generic format
		b.WriteString(fmt.Sprintf("Title: %s\n", details.Title))
		if len(details.Genres) > 0 {
			b.WriteString(fmt.Sprintf("Genres: %s\n", strings.Join(details.Genres, ", ")))
		}
		if details.Plot != "" {
			b.WriteString(fmt.Sprintf("Description: %s\n", details.Plot))
		}
	}

	return b.String()
}

// parseTags extracts mood tags from LLM response.
// Handles both JSON array format and comma-separated format.
func (s *MoodTagService) parseTags(response string) []string {
	response = strings.TrimSpace(response)

	// Try JSON array first
	var tags []string
	if err := json.Unmarshal([]byte(response), &tags); err == nil {
		return s.normalizeTags(tags)
	}

	// Try comma-separated format
	// Remove common prefixes like "Tags:" or "Mood tags:"
	response = strings.TrimPrefix(response, "Tags:")
	response = strings.TrimPrefix(response, "Mood tags:")
	response = strings.TrimPrefix(response, "Mood:")
	response = strings.TrimSpace(response)

	// Split by comma or newline
	var parts []string
	if strings.Contains(response, ",") {
		parts = strings.Split(response, ",")
	} else {
		parts = strings.Split(response, "\n")
	}

	for _, part := range parts {
		tag := strings.TrimSpace(part)
		// Remove bullet points, numbers, etc.
		tag = strings.TrimLeft(tag, "•-*0123456789. ")
		if tag != "" {
			tags = append(tags, tag)
		}
	}

	return s.normalizeTags(tags)
}

// normalizeTags cleans and limits tags.
func (s *MoodTagService) normalizeTags(tags []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, 5)

	for _, tag := range tags {
		// Lowercase and trim
		tag = strings.ToLower(strings.TrimSpace(tag))

		// Skip empty or very long tags
		if tag == "" || len(tag) > 30 {
			continue
		}

		// Skip duplicates
		if seen[tag] {
			continue
		}
		seen[tag] = true

		result = append(result, tag)

		// Limit to 5 tags
		if len(result) >= 5 {
			break
		}
	}

	return result
}

// moodTagSystemPrompt is the system prompt for mood tag generation.
//
//nolint:lll // Long prompt string is acceptable for LLM system prompts.
const moodTagSystemPrompt = `You are a media mood expert helping users find what to watch based on their mood.

Generate 3-5 SPECIFIC mood tags that capture what makes this content distinctive. Avoid generic tags.

PICK FROM THESE TAGS (use these exact phrases):

EMOTIONAL TONE:
- cozy, heartwarming, bittersweet, melancholic, hopeful, cathartic
- dark, bleak, unsettling, eerie, haunting
- uplifting, feel-good, triumphant, inspiring
- tearjerker, gut-wrenching, emotional rollercoaster
- lighthearted, fun, playful, whimsical

PACING & ENERGY:
- slow-burn, meditative, languid
- fast-paced, frenetic, relentless, action-packed
- steady, measured, deliberate

TENSION & THRILLS:
- edge-of-your-seat, nail-biting, white-knuckle
- slow-building dread, psychological tension
- cat-and-mouse, paranoid, twist-ending

COMEDY STYLES:
- cringe comedy, absurdist, satirical, dry humor, slapstick
- darkly funny, witty banter, workplace comedy
- feel-good comedy, buddy comedy, rom-com

INTELLECTUAL ENGAGEMENT:
- mind-bending, cerebral, thought-provoking
- morally complex, philosophical, puzzle-box

WORLD & ATMOSPHERE:
- immersive world, escapist, epic scope
- gritty, grounded, intimate
- nostalgic, retro, throwback

SOCIAL VIEWING:
- comfort watch, binge-worthy, background show
- watch-with-friends, crowd-pleaser, date night
- family movie night, late-night viewing

MUSIC-SPECIFIC (for albums/artists only):
- chill vibes, energetic, anthemic, atmospheric
- melancholic, euphoric, introspective
- workout music, study music, party music
- road trip, summer vibes, rainy day

NEVER USE THESE GENERIC TAGS:
- dark, tense, intense, comedic, dramatic, suspenseful, thrilling
- good, bad, interesting, nice, great, amazing, awesome
- action, drama, comedy, horror (these are genres, not moods)
- movie, film, show, series, album (these are media types)

BAD EXAMPLES (what NOT to return):
- "The Matrix" → ["action", "sci-fi", "intense"] ❌ (genres and generic)
- "Titanic" → ["romantic", "dramatic", "sad"] ❌ (too vague)
- "any movie" → ["entertaining", "watchable", "good"] ❌ (meaningless)

GOOD EXAMPLES:
- "The Office" → ["cringe comedy", "workplace comedy", "comfort watch", "heartwarming"]
- "Breaking Bad" → ["slow-burn", "morally complex", "edge-of-your-seat", "gut-wrenching"]
- "Fleabag" → ["darkly funny", "bittersweet", "cathartic", "intimate"]
- "Interstellar" → ["mind-bending", "epic scope", "tearjerker", "immersive world"]
- "Bridesmaids" → ["feel-good comedy", "watch-with-friends", "crowd-pleaser"]
- "Stranger Things" → ["nostalgic", "binge-worthy", "edge-of-your-seat", "immersive world"]
- "Radiohead - OK Computer" → ["melancholic", "atmospheric", "introspective", "cerebral"]
- "Daft Punk - Discovery" → ["euphoric", "energetic", "nostalgic", "anthemic"]

Respond with ONLY a JSON array of 3-5 tags. No explanations.`
