package settings

import "errors"

// Setting validation errors
var (
	ErrKeyEmpty        = errors.New("setting key cannot be empty")
	ErrValueEmpty      = errors.New("setting value cannot be empty")
	ErrInvalidValue    = errors.New("invalid setting value")
	ErrInvalidType     = errors.New("invalid value type for setting")
	ErrValidationMin   = errors.New("value is below minimum")
	ErrValidationMax   = errors.New("value is above maximum")
	ErrValidationRegex = errors.New("value does not match required pattern")
)

// Repository errors
var (
	ErrSettingNotFound = errors.New("setting not found")
	ErrUnknownSetting  = errors.New("unknown setting key")
)
