package settings

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	settingsDomain "github.com/mantonx/viewra/internal/domain/settings"
	"github.com/mantonx/viewra/internal/infrastructure/crypto"
	"github.com/mantonx/viewra/internal/infrastructure/system"
)

// Service handles settings operations with in-memory caching.
type Service struct {
	systemRepo settingsDomain.SystemRepository
	userRepo   settingsDomain.UserRepository
	encryptor  *crypto.Encryptor

	// In-memory cache
	mu          sync.RWMutex
	systemCache map[string]*settingsDomain.SystemSetting
	userCache   map[string]map[string]*settingsDomain.UserSetting // userID -> key -> setting
	cacheWarmed bool

	// System profile for detected values
	systemProfile *system.Profile
}

// NewService creates a new settings service.
func NewService(
	systemRepo settingsDomain.SystemRepository,
	userRepo settingsDomain.UserRepository,
	encryptor *crypto.Encryptor,
) *Service {
	return &Service{
		systemRepo:  systemRepo,
		userRepo:    userRepo,
		encryptor:   encryptor,
		systemCache: make(map[string]*settingsDomain.SystemSetting),
		userCache:   make(map[string]map[string]*settingsDomain.UserSetting),
	}
}

// SetSystemProfile sets the detected system profile for read-only settings.
func (s *Service) SetSystemProfile(profile *system.Profile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.systemProfile = profile
}

// GetSystemProfile returns the detected system profile.
func (s *Service) GetSystemProfile() *system.Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.systemProfile
}

// WarmCache loads all system settings into the cache.
// Should be called at server startup.
func (s *Service) WarmCache(ctx context.Context) error {
	settings, err := s.systemRepo.GetAll(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, setting := range settings {
		s.systemCache[setting.Key] = setting
	}
	s.cacheWarmed = true

	return nil
}

// GetSystem retrieves a system setting by key.
// Uses cache if available, otherwise reads from database.
func (s *Service) GetSystem(ctx context.Context, key string) (*settingsDomain.SystemSetting, error) {
	// Check cache first
	s.mu.RLock()
	if setting, ok := s.systemCache[key]; ok {
		s.mu.RUnlock()
		return setting, nil
	}
	s.mu.RUnlock()

	// Read from database
	setting, err := s.systemRepo.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	// Update cache
	s.mu.Lock()
	s.systemCache[key] = setting
	s.mu.Unlock()

	return setting, nil
}

// GetSystemValue retrieves a system setting value by key.
// Returns the default value from definition if not found.
// Sensitive values are decrypted automatically.
func (s *Service) GetSystemValue(ctx context.Context, key string) (any, error) {
	setting, err := s.GetSystem(ctx, key)
	if err != nil {
		if err == settingsDomain.ErrSettingNotFound {
			// Return default from definition
			def := settingsDomain.GetSystemDefinition(key)
			if def != nil {
				return def.Default, nil
			}
			return nil, settingsDomain.ErrUnknownSetting
		}
		return nil, err
	}

	// Decode based on type
	var value any
	switch setting.ValueType {
	case settingsDomain.TypeString:
		value = setting.GetString()
	case settingsDomain.TypeInt:
		value = setting.GetInt()
	case settingsDomain.TypeBool:
		value = setting.GetBool()
	case settingsDomain.TypeJSON:
		var v any
		if err := setting.GetJSON(&v); err != nil {
			return nil, err
		}
		value = v
	default:
		value = setting.Value
	}

	// Decrypt sensitive values
	def := settingsDomain.GetSystemDefinition(key)
	if def != nil && def.Sensitive && s.encryptor != nil {
		if strVal, ok := value.(string); ok && crypto.IsEncrypted(strVal) {
			decrypted, decErr := s.encryptor.Decrypt(strVal)
			if decErr != nil {
				return nil, fmt.Errorf("failed to decrypt sensitive value: %w", decErr)
			}
			return decrypted, nil
		}
	}

	return value, nil
}

// SetSystem creates or updates a system setting.
func (s *Service) SetSystem(ctx context.Context, key string, value any, updatedBy string) error {
	// Get definition to determine type
	def := settingsDomain.GetSystemDefinition(key)
	if def == nil {
		return settingsDomain.ErrUnknownSetting
	}

	// Encode value
	encoded, err := settingsDomain.EncodeValue(value)
	if err != nil {
		return settingsDomain.ErrInvalidValue
	}

	// Encrypt sensitive values
	if def.Sensitive && s.encryptor != nil {
		// For strings, encrypt the raw value before JSON encoding
		if strVal, ok := value.(string); ok && strVal != "" {
			encrypted, encErr := s.encryptor.Encrypt(strVal)
			if encErr != nil {
				return fmt.Errorf("failed to encrypt sensitive value: %w", encErr)
			}
			encoded, err = settingsDomain.EncodeValue(encrypted)
			if err != nil {
				return settingsDomain.ErrInvalidValue
			}
		}
	}

	setting := &settingsDomain.SystemSetting{
		Key:         key,
		Value:       encoded,
		ValueType:   def.Type,
		Category:    def.Category,
		Description: def.Description,
		UpdatedAt:   time.Now(),
		UpdatedBy:   updatedBy,
	}

	// Write to database
	if err := s.systemRepo.Set(ctx, setting); err != nil {
		return err
	}

	// Invalidate cache
	s.mu.Lock()
	s.systemCache[key] = setting
	s.mu.Unlock()

	return nil
}

// GetSystemByCategory retrieves all system settings in a category.
func (s *Service) GetSystemByCategory(ctx context.Context, category settingsDomain.Category) (map[string]any, error) {
	allSettings, err := s.systemRepo.GetByCategory(ctx, category)
	if err != nil {
		return nil, err
	}

	result := make(map[string]any)
	for _, setting := range allSettings {
		switch setting.ValueType {
		case settingsDomain.TypeString:
			result[setting.Key] = setting.GetString()
		case settingsDomain.TypeInt:
			result[setting.Key] = setting.GetInt()
		case settingsDomain.TypeBool:
			result[setting.Key] = setting.GetBool()
		case settingsDomain.TypeJSON:
			var v any
			if err := setting.GetJSON(&v); err == nil {
				result[setting.Key] = v
			}
		}
	}

	return result, nil
}

// GetAllSystem retrieves all system settings.
func (s *Service) GetAllSystem(ctx context.Context) ([]*settingsDomain.SystemSetting, error) {
	return s.systemRepo.GetAll(ctx)
}

// GetUser retrieves a user setting by user ID and key.
func (s *Service) GetUser(ctx context.Context, userID, key string) (*settingsDomain.UserSetting, error) {
	// Check cache first
	s.mu.RLock()
	if userSettings, ok := s.userCache[userID]; ok {
		if setting, ok := userSettings[key]; ok {
			s.mu.RUnlock()
			return setting, nil
		}
	}
	s.mu.RUnlock()

	// Read from database
	setting, err := s.userRepo.Get(ctx, userID, key)
	if err != nil {
		return nil, err
	}

	// Update cache
	s.mu.Lock()
	if s.userCache[userID] == nil {
		s.userCache[userID] = make(map[string]*settingsDomain.UserSetting)
	}
	s.userCache[userID][key] = setting
	s.mu.Unlock()

	return setting, nil
}

// GetUserValue retrieves a user setting value, falling back to default.
func (s *Service) GetUserValue(ctx context.Context, userID, key string) (any, error) {
	setting, err := s.GetUser(ctx, userID, key)
	if err != nil {
		if err == settingsDomain.ErrSettingNotFound {
			// Return default from definition
			def := settingsDomain.GetUserDefinition(key)
			if def != nil {
				return def.Default, nil
			}
			return nil, settingsDomain.ErrUnknownSetting
		}
		return nil, err
	}

	// Get type from definition
	def := settingsDomain.GetUserDefinition(key)
	if def == nil {
		return setting.Value, nil
	}

	// Decode based on type
	switch def.Type {
	case settingsDomain.TypeString:
		return setting.GetString(), nil
	case settingsDomain.TypeInt:
		return setting.GetInt(), nil
	case settingsDomain.TypeBool:
		return setting.GetBool(), nil
	case settingsDomain.TypeJSON:
		var v any
		if err := setting.GetJSON(&v); err != nil {
			return nil, err
		}
		return v, nil
	}

	return setting.Value, nil
}

// SetUser creates or updates a user setting.
func (s *Service) SetUser(ctx context.Context, userID, key string, value any) error {
	// Get definition to validate
	def := settingsDomain.GetUserDefinition(key)
	if def == nil {
		return settingsDomain.ErrUnknownSetting
	}

	// Encode value
	encoded, err := settingsDomain.EncodeValue(value)
	if err != nil {
		return settingsDomain.ErrInvalidValue
	}

	setting := &settingsDomain.UserSetting{
		UserID:    userID,
		Key:       key,
		Value:     encoded,
		UpdatedAt: time.Now(),
	}

	// Write to database
	if err := s.userRepo.Set(ctx, setting); err != nil {
		return err
	}

	// Update cache
	s.mu.Lock()
	if s.userCache[userID] == nil {
		s.userCache[userID] = make(map[string]*settingsDomain.UserSetting)
	}
	s.userCache[userID][key] = setting
	s.mu.Unlock()

	return nil
}

// DeleteUser removes a user setting (resets to default).
func (s *Service) DeleteUser(ctx context.Context, userID, key string) error {
	if err := s.userRepo.Delete(ctx, userID, key); err != nil {
		return err
	}

	// Invalidate cache
	s.mu.Lock()
	if userSettings, ok := s.userCache[userID]; ok {
		delete(userSettings, key)
	}
	s.mu.Unlock()

	return nil
}

// GetAllUser retrieves all settings for a user.
func (s *Service) GetAllUser(ctx context.Context, userID string) ([]*settingsDomain.UserSetting, error) {
	return s.userRepo.GetAll(ctx, userID)
}

// GetEffective retrieves the effective value for a setting.
// Returns user override if exists, otherwise system default.
func (s *Service) GetEffective(ctx context.Context, userID, key string) (any, error) {
	// Check if it's a user setting
	if def := settingsDomain.GetUserDefinition(key); def != nil {
		return s.GetUserValue(ctx, userID, key)
	}

	// Check if it's a system setting
	if def := settingsDomain.GetSystemDefinition(key); def != nil {
		return s.GetSystemValue(ctx, key)
	}

	return nil, settingsDomain.ErrUnknownSetting
}

// Reload clears the cache and reloads from database.
func (s *Service) Reload(ctx context.Context) error {
	s.mu.Lock()
	s.systemCache = make(map[string]*settingsDomain.SystemSetting)
	s.userCache = make(map[string]map[string]*settingsDomain.UserSetting)
	s.cacheWarmed = false
	s.mu.Unlock()

	return s.WarmCache(ctx)
}

// GetSystemDefinitions returns all system setting definitions.
func (s *Service) GetSystemDefinitions() []settingsDomain.Definition {
	return settingsDomain.SystemSettingDefinitions
}

// GetUserDefinitions returns all user setting definitions.
func (s *Service) GetUserDefinitions() []settingsDomain.Definition {
	return settingsDomain.UserSettingDefinitions
}

// GetSystemValueMasked retrieves a system setting value with sensitive values masked.
// Use this for displaying values in the UI where full decryption is not needed.
func (s *Service) GetSystemValueMasked(ctx context.Context, key string) (any, error) {
	def := settingsDomain.GetSystemDefinition(key)
	if def == nil {
		return nil, settingsDomain.ErrUnknownSetting
	}

	// For non-sensitive settings, return the normal value
	if !def.Sensitive {
		return s.GetSystemValue(ctx, key)
	}

	// Get the raw (possibly encrypted) value from the database
	setting, err := s.GetSystem(ctx, key)
	if err != nil {
		if err == settingsDomain.ErrSettingNotFound {
			return def.Default, nil
		}
		return nil, err
	}

	// Get the string value and mask it
	strVal := setting.GetString()
	if strVal == "" {
		return "", nil
	}

	// Return masked value
	return crypto.MaskAPIKey(strVal), nil
}

// IsSensitiveSetting returns true if the setting is marked as sensitive.
func (s *Service) IsSensitiveSetting(key string) bool {
	def := settingsDomain.GetSystemDefinition(key)
	return def != nil && def.Sensitive
}

// GetEffectiveSystemValue resolves a system setting with full source tracking.
// Resolution order: env var > database > detected value > default
func (s *Service) GetEffectiveSystemValue(ctx context.Context, key string) (settingsDomain.EffectiveValue, error) {
	def := settingsDomain.GetSystemDefinition(key)
	if def == nil {
		return settingsDomain.EffectiveValue{}, settingsDomain.ErrUnknownSetting
	}

	// 1. Check if it's a read-only detected value
	if def.ReadOnly {
		detectedValue := s.getDetectedValue(key)
		return settingsDomain.EffectiveValue{
			Value:    detectedValue,
			Source:   settingsDomain.SourceDetected,
			Locked:   true,
			ReadOnly: true,
		}, nil
	}

	// 2. Check if env var is set (takes precedence)
	if def.EnvVar != "" {
		if envVal := os.Getenv(def.EnvVar); envVal != "" {
			parsedValue := s.parseEnvValue(envVal, def.Type)
			return settingsDomain.EffectiveValue{
				Value:  parsedValue,
				Source: settingsDomain.SourceEnvVar,
				EnvVar: def.EnvVar,
				Locked: true,
			}, nil
		}
	}

	// 3. Check database value
	setting, err := s.GetSystem(ctx, key)
	if err == nil && setting != nil {
		var value any
		switch setting.ValueType {
		case settingsDomain.TypeString:
			value = setting.GetString()
		case settingsDomain.TypeInt:
			value = setting.GetInt()
		case settingsDomain.TypeBool:
			value = setting.GetBool()
		case settingsDomain.TypeJSON:
			var v any
			if err := setting.GetJSON(&v); err == nil {
				value = v
			}
		default:
			value = setting.Value
		}

		// For sensitive values, mask the value for display
		if def.Sensitive {
			if strVal, ok := value.(string); ok && strVal != "" {
				value = crypto.MaskAPIKey(strVal)
			}
		}

		return settingsDomain.EffectiveValue{
			Value:  value,
			Source: settingsDomain.SourceDatabase,
			Locked: false,
		}, nil
	}

	// 4. Return default
	return settingsDomain.EffectiveValue{
		Value:  def.Default,
		Source: settingsDomain.SourceDefault,
		Locked: false,
	}, nil
}

// GetAllEffectiveSystemValues returns all system settings with their effective values.
// Note: AI settings are excluded as they have a dedicated API endpoint.
func (s *Service) GetAllEffectiveSystemValues(ctx context.Context) (map[string]settingsDomain.EffectiveValue, error) {
	result := make(map[string]settingsDomain.EffectiveValue)

	for _, def := range settingsDomain.SystemSettingDefinitions {
		// Skip AI settings - they have their own dedicated endpoint
		if def.Category == settingsDomain.CategoryAI {
			continue
		}
		effectiveValue, err := s.GetEffectiveSystemValue(ctx, def.Key)
		if err != nil {
			continue // Skip settings that fail to resolve
		}
		result[def.Key] = effectiveValue
	}

	return result, nil
}

// IsEnvVarLocked checks if a setting is locked by an environment variable.
func (s *Service) IsEnvVarLocked(key string) bool {
	def := settingsDomain.GetSystemDefinition(key)
	if def == nil || def.EnvVar == "" {
		return false
	}
	return os.Getenv(def.EnvVar) != ""
}

// getDetectedValue returns the detected value for read-only system settings.
func (s *Service) getDetectedValue(key string) any {
	s.mu.RLock()
	profile := s.systemProfile
	s.mu.RUnlock()

	if profile == nil {
		return nil
	}

	switch key {
	case "system.cpu_model":
		return profile.CPU.Model
	case "system.cpu_cores":
		return fmt.Sprintf("%d physical / %d logical", profile.CPU.NumPhysical, profile.CPU.NumCPU)
	case "system.memory_total":
		gb := float64(profile.Memory.TotalBytes) / (1024 * 1024 * 1024)
		return fmt.Sprintf("%.1f GB", gb)
	case "system.gpu_type":
		if !profile.GPU.Available {
			return "None detected"
		}
		return fmt.Sprintf("%s (%s)", profile.GPU.Type, profile.GetHardwareAccelType())
	case "system.gpu_devices":
		return profile.GPU.DeviceNames
	case "transcoding.hw_accel_detected":
		// Return the actual detected hardware acceleration type
		hwAccel := profile.GetHardwareAccelType()
		if hwAccel == "" || hwAccel == "none" {
			return "none (CPU encoding)"
		}
		// Map to human-readable names
		switch hwAccel {
		case "nvenc":
			return "NVIDIA NVENC"
		case "qsv":
			return "Intel QuickSync"
		case "vaapi":
			return "VAAPI"
		default:
			return hwAccel
		}
	case "server.version":
		return "0.0.1" // TODO: inject from build
	case "server.environment":
		env := os.Getenv("ENVIRONMENT")
		if env == "" {
			return "development"
		}
		return env
	}

	return nil
}

// parseEnvValue converts an environment variable string to the appropriate type.
func (s *Service) parseEnvValue(envVal string, valueType settingsDomain.ValueType) any {
	switch valueType {
	case settingsDomain.TypeBool:
		return envVal == "true" || envVal == "1"
	case settingsDomain.TypeInt:
		if v, err := strconv.Atoi(envVal); err == nil {
			return v
		}
		return 0
	case settingsDomain.TypeJSON:
		// For JSON, return as string (caller should parse if needed)
		return envVal
	default:
		return envVal
	}
}
