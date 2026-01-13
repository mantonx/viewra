package internal

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// CurrentBoostConfigVersion is the current version of the boost config schema.
// Increment when adding/changing keys to trigger merge with defaults.
const CurrentBoostConfigVersion = 2

// CurrentStudiosConfigVersion is the current version of the studios config schema.
const CurrentStudiosConfigVersion = 1

// CurrentLanguagesConfigVersion is the current version of the languages config schema.
const CurrentLanguagesConfigVersion = 1

// BoostConfig holds all boost weights for keyword matching and ranking.
type BoostConfig struct {
	ConfigVersion int          `yaml:"config_version"`
	Boosts        BoostWeights `yaml:"boosts"`
	Quality       QualityBoost `yaml:"quality"`
	Diversity     Diversity    `yaml:"diversity"`
}

// BoostWeights contains individual boost/penalty values for different match types.
type BoostWeights struct {
	DirectorMatch                  float32 `yaml:"director_match"`
	DirectorMismatchPenalty        float32 `yaml:"director_mismatch_penalty"`
	ActorMatch                     float32 `yaml:"actor_match"`
	ActorMismatchPenalty           float32 `yaml:"actor_mismatch_penalty"`
	WriterMatch                    float32 `yaml:"writer_match"`
	WriterMismatchPenalty          float32 `yaml:"writer_mismatch_penalty"`
	ProducerMatch                  float32 `yaml:"producer_match"`
	ProducerMismatchPenalty        float32 `yaml:"producer_mismatch_penalty"`
	StudioMatch                    float32 `yaml:"studio_match"`
	StudioMismatchPenalty          float32 `yaml:"studio_mismatch_penalty"`
	PersonDirectorMatch            float32 `yaml:"person_director_match"`
	PersonWriterMatch              float32 `yaml:"person_writer_match"`
	PersonStudioMatch              float32 `yaml:"person_studio_match"`
	PersonProducerMatch            float32 `yaml:"person_producer_match"`
	PersonCastMatch                float32 `yaml:"person_cast_match"`
	PersonCastPenalty              float32 `yaml:"person_cast_penalty"`
	PersonNotFoundPenalty          float32 `yaml:"person_not_found_penalty"`
	LanguageMatch                  float32 `yaml:"language_match"`
	LanguageMismatchPenalty        float32 `yaml:"language_mismatch_penalty"`
	GenreMatch                     float32 `yaml:"genre_match"`
	GenreMismatchPenalty           float32 `yaml:"genre_mismatch_penalty"`
	ComposerMatch                  float32 `yaml:"composer_match"`
	ComposerMismatchPenalty        float32 `yaml:"composer_mismatch_penalty"`
	CinematographerMatch           float32 `yaml:"cinematographer_match"`
	CinematographerMismatchPenalty float32 `yaml:"cinematographer_mismatch_penalty"`
}

// QualityBoost configures quality signal weights.
type QualityBoost struct {
	Enabled  bool    `yaml:"enabled"`
	MaxBoost float32 `yaml:"max_boost"`
	MinVotes int64   `yaml:"min_votes"`
}

// Diversity configures diversity penalties.
type Diversity struct {
	SameDirectorPenalty float32 `yaml:"same_director_penalty"`
	SameDecadePenalty   float32 `yaml:"same_decade_penalty"`
	SameGenrePenalty    float32 `yaml:"same_genre_penalty"`
}

// StudiosConfig holds the list of known studios for intent detection.
type StudiosConfig struct {
	ConfigVersion int      `yaml:"config_version"`
	Studios       []string `yaml:"studios"`
}

// LanguagesConfig holds language name mappings.
type LanguagesConfig struct {
	ConfigVersion int               `yaml:"config_version"`
	Languages     map[string]string `yaml:"languages"`
}

// RankingConfig holds all loaded ranking configuration.
type RankingConfig struct {
	Boosts    *BoostConfig
	Studios   map[string]bool
	Languages map[string]string
}

// getDefaultBoostConfig returns the default boost configuration.
func getDefaultBoostConfig() *BoostConfig {
	return &BoostConfig{
		ConfigVersion: CurrentBoostConfigVersion,
		Boosts: BoostWeights{
			DirectorMatch:                  0.55,
			DirectorMismatchPenalty:        0.35,
			ActorMatch:                     0.50,
			ActorMismatchPenalty:           0.30,
			WriterMatch:                    0.45,
			WriterMismatchPenalty:          0.25,
			ProducerMatch:                  0.40,
			ProducerMismatchPenalty:        0.20,
			StudioMatch:                    0.50,
			StudioMismatchPenalty:          0.30,
			PersonDirectorMatch:            0.60,
			PersonWriterMatch:              0.50,
			PersonStudioMatch:              0.55,
			PersonProducerMatch:            0.35,
			PersonCastMatch:                0.25,
			PersonCastPenalty:              0.15,
			PersonNotFoundPenalty:          0.45,
			LanguageMatch:                  0.55,
			LanguageMismatchPenalty:        0.35,
			GenreMatch:                     0.20,
			GenreMismatchPenalty:           0.45,
			ComposerMatch:                  0.50,
			ComposerMismatchPenalty:        0.30,
			CinematographerMatch:           0.50,
			CinematographerMismatchPenalty: 0.30,
		},
		Quality: QualityBoost{
			Enabled:  true,
			MaxBoost: 0.15,
			MinVotes: 100,
		},
		Diversity: Diversity{
			SameDirectorPenalty: 0.03,
			SameDecadePenalty:   0.01,
			SameGenrePenalty:    0.01,
		},
	}
}

// getDefaultStudios returns the default list of known studios.
func getDefaultStudios() []string {
	return []string{
		"pixar", "disney", "marvel", "a24", "ghibli", "dreamworks",
		"warner", "universal", "paramount", "sony", "fox", "mgm",
		"lionsgate", "miramax", "blumhouse", "netflix", "hbo",
		"amazon", "apple", "hulu", "lucasfilm", "dc", "legendary",
		"annapurna", "neon", "searchlight", "focus", "studio ghibli",
	}
}

// getDefaultLanguages returns the default language mappings.
func getDefaultLanguages() map[string]string {
	return map[string]string{
		"french":     "french",
		"korean":     "korean",
		"japanese":   "japanese",
		"chinese":    "chinese",
		"spanish":    "spanish",
		"italian":    "italian",
		"german":     "german",
		"russian":    "russian",
		"indian":     "hindi",
		"bollywood":  "hindi",
		"swedish":    "swedish",
		"danish":     "danish",
		"norwegian":  "norwegian",
		"thai":       "thai",
		"turkish":    "turkish",
		"portuguese": "portuguese",
		"arabic":     "arabic",
		"polish":     "polish",
		"dutch":      "dutch",
		"greek":      "greek",
		"czech":      "czech",
		"hungarian":  "hungarian",
		"romanian":   "romanian",
		"vietnamese": "vietnamese",
		"indonesian": "indonesian",
		"filipino":   "filipino",
		"k-drama":    "korean",
		"kdrama":     "korean",
		"j-drama":    "japanese",
		"jdrama":     "japanese",
		"c-drama":    "chinese",
		"cdrama":     "chinese",
		"anime":      "japanese",
	}
}

// LoadRankingConfig loads all ranking configuration from the data directory.
// If config files don't exist, they are created with defaults.
func LoadRankingConfig(dataDir string, logger *slog.Logger) (*RankingConfig, error) {
	config := &RankingConfig{}

	// Load boost config
	boosts, err := loadBoostConfig(dataDir, logger)
	if err != nil {
		logger.Warn("failed to load boost config, using defaults", "error", err)
		boosts = getDefaultBoostConfig()
	}
	config.Boosts = boosts

	// Load studios config
	studios, err := loadStudios(dataDir, logger)
	if err != nil {
		logger.Warn("failed to load studios config, using defaults", "error", err)
		studios = studiosToMap(getDefaultStudios())
	}
	config.Studios = studios

	// Load languages config
	languages, err := loadLanguages(dataDir, logger)
	if err != nil {
		logger.Warn("failed to load languages config, using defaults", "error", err)
		languages = getDefaultLanguages()
	}
	config.Languages = languages

	return config, nil
}

// loadBoostConfig loads boost configuration from YAML file.
func loadBoostConfig(dataDir string, logger *slog.Logger) (*BoostConfig, error) {
	path := filepath.Join(dataDir, "boosts.yaml")
	defaults := getDefaultBoostConfig()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// First run: create default config file
		if err := writeBoostConfig(path, defaults); err != nil {
			logger.Warn("failed to write default boost config", "error", err)
		} else {
			logger.Info("created default boost config", "path", path)
		}
		return defaults, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read boost config: %w", err)
	}

	var userConfig BoostConfig
	if err := yaml.Unmarshal(data, &userConfig); err != nil {
		return nil, fmt.Errorf("parse boost config: %w", err)
	}

	// Check version and merge with defaults
	if userConfig.ConfigVersion < defaults.ConfigVersion {
		logger.Warn("boost config version outdated, merging with defaults",
			"user_version", userConfig.ConfigVersion,
			"current_version", defaults.ConfigVersion)
	}

	merged := mergeBoostConfigs(defaults, &userConfig)
	return merged, nil
}

// mergeBoostConfigs merges user config with defaults, preserving user values.
func mergeBoostConfigs(defaults, user *BoostConfig) *BoostConfig {
	result := *defaults // Start with all defaults

	// Override with user values where present (non-zero)
	if user.Boosts.DirectorMatch != 0 {
		result.Boosts.DirectorMatch = user.Boosts.DirectorMatch
	}
	if user.Boosts.DirectorMismatchPenalty != 0 {
		result.Boosts.DirectorMismatchPenalty = user.Boosts.DirectorMismatchPenalty
	}
	if user.Boosts.ActorMatch != 0 {
		result.Boosts.ActorMatch = user.Boosts.ActorMatch
	}
	if user.Boosts.ActorMismatchPenalty != 0 {
		result.Boosts.ActorMismatchPenalty = user.Boosts.ActorMismatchPenalty
	}
	if user.Boosts.WriterMatch != 0 {
		result.Boosts.WriterMatch = user.Boosts.WriterMatch
	}
	if user.Boosts.WriterMismatchPenalty != 0 {
		result.Boosts.WriterMismatchPenalty = user.Boosts.WriterMismatchPenalty
	}
	if user.Boosts.ProducerMatch != 0 {
		result.Boosts.ProducerMatch = user.Boosts.ProducerMatch
	}
	if user.Boosts.ProducerMismatchPenalty != 0 {
		result.Boosts.ProducerMismatchPenalty = user.Boosts.ProducerMismatchPenalty
	}
	if user.Boosts.StudioMatch != 0 {
		result.Boosts.StudioMatch = user.Boosts.StudioMatch
	}
	if user.Boosts.StudioMismatchPenalty != 0 {
		result.Boosts.StudioMismatchPenalty = user.Boosts.StudioMismatchPenalty
	}
	if user.Boosts.PersonDirectorMatch != 0 {
		result.Boosts.PersonDirectorMatch = user.Boosts.PersonDirectorMatch
	}
	if user.Boosts.PersonWriterMatch != 0 {
		result.Boosts.PersonWriterMatch = user.Boosts.PersonWriterMatch
	}
	if user.Boosts.PersonStudioMatch != 0 {
		result.Boosts.PersonStudioMatch = user.Boosts.PersonStudioMatch
	}
	if user.Boosts.PersonProducerMatch != 0 {
		result.Boosts.PersonProducerMatch = user.Boosts.PersonProducerMatch
	}
	if user.Boosts.PersonCastMatch != 0 {
		result.Boosts.PersonCastMatch = user.Boosts.PersonCastMatch
	}
	if user.Boosts.PersonCastPenalty != 0 {
		result.Boosts.PersonCastPenalty = user.Boosts.PersonCastPenalty
	}
	if user.Boosts.PersonNotFoundPenalty != 0 {
		result.Boosts.PersonNotFoundPenalty = user.Boosts.PersonNotFoundPenalty
	}
	if user.Boosts.LanguageMatch != 0 {
		result.Boosts.LanguageMatch = user.Boosts.LanguageMatch
	}
	if user.Boosts.LanguageMismatchPenalty != 0 {
		result.Boosts.LanguageMismatchPenalty = user.Boosts.LanguageMismatchPenalty
	}
	if user.Boosts.GenreMatch != 0 {
		result.Boosts.GenreMatch = user.Boosts.GenreMatch
	}
	if user.Boosts.GenreMismatchPenalty != 0 {
		result.Boosts.GenreMismatchPenalty = user.Boosts.GenreMismatchPenalty
	}
	if user.Boosts.ComposerMatch != 0 {
		result.Boosts.ComposerMatch = user.Boosts.ComposerMatch
	}
	if user.Boosts.ComposerMismatchPenalty != 0 {
		result.Boosts.ComposerMismatchPenalty = user.Boosts.ComposerMismatchPenalty
	}
	if user.Boosts.CinematographerMatch != 0 {
		result.Boosts.CinematographerMatch = user.Boosts.CinematographerMatch
	}
	if user.Boosts.CinematographerMismatchPenalty != 0 {
		result.Boosts.CinematographerMismatchPenalty = user.Boosts.CinematographerMismatchPenalty
	}

	// Quality settings - check explicitly since false/0 are valid values
	// We use a simple heuristic: if user config version > 0, they've customized it
	if user.ConfigVersion > 0 {
		result.Quality.Enabled = user.Quality.Enabled
		if user.Quality.MaxBoost != 0 {
			result.Quality.MaxBoost = user.Quality.MaxBoost
		}
		if user.Quality.MinVotes != 0 {
			result.Quality.MinVotes = user.Quality.MinVotes
		}
	}

	// Diversity settings
	if user.Diversity.SameDirectorPenalty != 0 {
		result.Diversity.SameDirectorPenalty = user.Diversity.SameDirectorPenalty
	}
	if user.Diversity.SameDecadePenalty != 0 {
		result.Diversity.SameDecadePenalty = user.Diversity.SameDecadePenalty
	}
	if user.Diversity.SameGenrePenalty != 0 {
		result.Diversity.SameGenrePenalty = user.Diversity.SameGenrePenalty
	}

	// Update version to current
	result.ConfigVersion = defaults.ConfigVersion

	return &result
}

// writeBoostConfig writes boost configuration to YAML file.
func writeBoostConfig(path string, config *BoostConfig) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal boost config: %w", err)
	}

	header := []byte(`# Boost weights for keyword matching
# Edit these values to tune search ranking behavior
# See docs for detailed explanations of each weight

`)
	content := append(header, data...)

	return os.WriteFile(path, content, 0644)
}

// loadStudios loads studios configuration from YAML file.
func loadStudios(dataDir string, logger *slog.Logger) (map[string]bool, error) {
	path := filepath.Join(dataDir, "studios.yaml")
	defaults := getDefaultStudios()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// First run: create default config file
		if err := writeStudiosConfig(path, defaults); err != nil {
			logger.Warn("failed to write default studios config", "error", err)
		} else {
			logger.Info("created default studios config", "path", path)
		}
		return studiosToMap(defaults), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read studios config: %w", err)
	}

	var config StudiosConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse studios config: %w", err)
	}

	if config.ConfigVersion < CurrentStudiosConfigVersion {
		logger.Warn("studios config version outdated",
			"user_version", config.ConfigVersion,
			"current_version", CurrentStudiosConfigVersion)
	}

	return studiosToMap(config.Studios), nil
}

// studiosToMap converts a slice of studio names to a map for O(1) lookup.
func studiosToMap(studios []string) map[string]bool {
	m := make(map[string]bool, len(studios))
	for _, s := range studios {
		m[s] = true
	}
	return m
}

// writeStudiosConfig writes studios configuration to YAML file.
func writeStudiosConfig(path string, studios []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	config := StudiosConfig{
		ConfigVersion: CurrentStudiosConfigVersion,
		Studios:       studios,
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal studios config: %w", err)
	}

	header := []byte(`# Known studios for intent detection
# Add studio names here to enable "movies by [studio]" queries
# Names should be lowercase

`)
	content := append(header, data...)

	return os.WriteFile(path, content, 0644)
}

// loadLanguages loads languages configuration from YAML file.
func loadLanguages(dataDir string, logger *slog.Logger) (map[string]string, error) {
	path := filepath.Join(dataDir, "languages.yaml")
	defaults := getDefaultLanguages()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// First run: create default config file
		if err := writeLanguagesConfig(path, defaults); err != nil {
			logger.Warn("failed to write default languages config", "error", err)
		} else {
			logger.Info("created default languages config", "path", path)
		}
		return defaults, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read languages config: %w", err)
	}

	var config LanguagesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse languages config: %w", err)
	}

	if config.ConfigVersion < CurrentLanguagesConfigVersion {
		logger.Warn("languages config version outdated",
			"user_version", config.ConfigVersion,
			"current_version", CurrentLanguagesConfigVersion)
	}

	// Merge with defaults to ensure new languages are available
	merged := make(map[string]string)
	for k, v := range defaults {
		merged[k] = v
	}
	for k, v := range config.Languages {
		merged[k] = v
	}

	return merged, nil
}

// writeLanguagesConfig writes languages configuration to YAML file.
func writeLanguagesConfig(path string, languages map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	config := LanguagesConfig{
		ConfigVersion: CurrentLanguagesConfigVersion,
		Languages:     languages,
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal languages config: %w", err)
	}

	header := []byte(`# Language name mappings for intent detection
# Maps query terms to canonical language names
# Example: "bollywood" -> "hindi", "anime" -> "japanese"

`)
	content := append(header, data...)

	return os.WriteFile(path, content, 0644)
}
