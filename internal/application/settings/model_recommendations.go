package settings

import (
	"context"
	"fmt"
	"strings"

	"github.com/mantonx/viewra/internal/domain/ai"
)

const (
	GB = 1024 * 1024 * 1024
	MB = 1024 * 1024
)

// ModelSpec defines the requirements and metadata for a model.
type ModelSpec struct {
	ID          string
	Name        string
	SizeBytes   uint64
	Description string
	MinRAM      uint64 // Minimum RAM in bytes
	MinVRAM     uint64 // Minimum VRAM in bytes (0 = CPU-only is fine)
}

// RecommendedModel represents a model with recommendation status.
type RecommendedModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Size        string `json:"size"`
	SizeBytes   uint64 `json:"sizeBytes"`
	Description string `json:"description"`
	Recommended bool   `json:"recommended"`
}

// SystemInfo contains system resource information.
type SystemInfo struct {
	RAMBytes      uint64 `json:"ramBytes"`
	RAMFormatted  string `json:"ramFormatted"`
	VRAMBytes     uint64 `json:"vramBytes"`
	VRAMFormatted string `json:"vramFormatted"`
	HasGPU        bool   `json:"hasGpu"`
}

// ModelRecommendations contains recommended models with system info.
type ModelRecommendations struct {
	Models     []RecommendedModel `json:"models"`
	SystemInfo SystemInfo         `json:"systemInfo"`
}

// embeddingModels defines available embedding models ordered by quality.
var embeddingModels = []ModelSpec{
	{
		ID:          "nomic-embed-text",
		Name:        "Nomic Embed",
		SizeBytes:   274 * MB,
		Description: "Best balance of quality and speed",
		MinRAM:      2 * GB,
		MinVRAM:     0,
	},
	{
		ID:          "mxbai-embed-large",
		Name:        "MixedBread Large",
		SizeBytes:   670 * MB,
		Description: "Higher quality embeddings, needs more resources",
		MinRAM:      4 * GB,
		MinVRAM:     4 * GB,
	},
	{
		ID:          "bge-base-en-v1.5",
		Name:        "BGE Base",
		SizeBytes:   134 * MB,
		Description: "Good quality, English-optimized",
		MinRAM:      2 * GB,
		MinVRAM:     0,
	},
	{
		ID:          "all-minilm",
		Name:        "MiniLM",
		SizeBytes:   46 * MB,
		Description: "Smallest and fastest, good for limited resources",
		MinRAM:      1 * GB,
		MinVRAM:     0,
	},
}

// chatModels defines available chat models ordered by quality.
var chatModels = []ModelSpec{
	{
		ID:          "llama3.1:8b",
		Name:        "Llama 3.1 8B",
		SizeBytes:   4_700 * MB,
		Description: "Best quality for most systems",
		MinRAM:      8 * GB,
		MinVRAM:     6 * GB,
	},
	{
		ID:          "gemma2:2b",
		Name:        "Gemma 2 2B",
		SizeBytes:   1_600 * MB,
		Description: "Fast and lightweight, good for basic tasks",
		MinRAM:      4 * GB,
		MinVRAM:     0,
	},
	{
		ID:          "phi3:mini",
		Name:        "Phi-3 Mini",
		SizeBytes:   2_300 * MB,
		Description: "Microsoft's compact model, good reasoning",
		MinRAM:      4 * GB,
		MinVRAM:     0,
	},
	{
		ID:          "qwen2:1.5b",
		Name:        "Qwen2 1.5B",
		SizeBytes:   935 * MB,
		Description: "Smallest option, basic capabilities",
		MinRAM:      2 * GB,
		MinVRAM:     0,
	},
}

// ModelLister can list installed models.
type ModelLister interface {
	ListModels(ctx context.Context) ([]ai.ModelInfo, error)
}

// SystemInfoProvider provides system resource information.
type SystemInfoProvider func() (ramBytes, vramBytes uint64)

// ModelRecommendationService provides model recommendations based on system resources.
type ModelRecommendationService struct {
	getSystemInfo SystemInfoProvider
}

// NewModelRecommendationService creates a new model recommendation service.
func NewModelRecommendationService(systemInfoProvider SystemInfoProvider) *ModelRecommendationService {
	return &ModelRecommendationService{
		getSystemInfo: systemInfoProvider,
	}
}

// GetEmbeddingRecommendations returns recommended embedding models.
func (s *ModelRecommendationService) GetEmbeddingRecommendations(ctx context.Context, lister ModelLister) (*ModelRecommendations, error) {
	return s.getRecommendations(ctx, lister, embeddingModels)
}

// GetChatRecommendations returns recommended chat models.
func (s *ModelRecommendationService) GetChatRecommendations(ctx context.Context, lister ModelLister) (*ModelRecommendations, error) {
	return s.getRecommendations(ctx, lister, chatModels)
}

func (s *ModelRecommendationService) getRecommendations(ctx context.Context, lister ModelLister, specs []ModelSpec) (*ModelRecommendations, error) {
	var ramBytes, vramBytes uint64
	if s.getSystemInfo != nil {
		ramBytes, vramBytes = s.getSystemInfo()
	}

	// Get installed models to skip when recommending
	installedModels := make(map[string]bool)
	if lister != nil {
		if models, err := lister.ListModels(ctx); err == nil {
			for _, m := range models {
				installedModels[m.ID] = true
				// Also match base name without tag suffix
				if idx := strings.Index(m.ID, ":"); idx > 0 {
					installedModels[m.ID[:idx]] = true
				}
			}
		}
	}

	hasGPU := vramBytes > 0
	models := make([]RecommendedModel, 0, len(specs))

	// Find the best model that can run and isn't installed
	bestModelID := ""
	for _, spec := range specs {
		canRun := ramBytes >= spec.MinRAM && (spec.MinVRAM == 0 || vramBytes >= spec.MinVRAM)
		isInstalled := installedModels[spec.ID]
		if canRun && !isInstalled && bestModelID == "" {
			bestModelID = spec.ID
		}
	}

	for _, spec := range specs {
		models = append(models, RecommendedModel{
			ID:          spec.ID,
			Name:        spec.Name,
			Size:        formatBytes(spec.SizeBytes),
			SizeBytes:   spec.SizeBytes,
			Description: spec.Description,
			Recommended: spec.ID == bestModelID,
		})
	}

	return &ModelRecommendations{
		Models: models,
		SystemInfo: SystemInfo{
			RAMBytes:      ramBytes,
			RAMFormatted:  formatBytes(ramBytes),
			VRAMBytes:     vramBytes,
			VRAMFormatted: formatBytes(vramBytes),
			HasGPU:        hasGPU,
		},
	}, nil
}

func formatBytes(bytes uint64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)

	switch {
	case bytes >= tb:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(tb))
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%d MB", bytes/mb)
	case bytes >= kb:
		return fmt.Sprintf("%d KB", bytes/kb)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
