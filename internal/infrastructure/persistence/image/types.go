package image

import (
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// ========================================
// SQLite Mappers
// ========================================

// sqliteImageToDomain converts a SQLite database image to a domain image
func sqliteImageToDomain(dbImage sqlc_sqlite.MediaImage) *images.Image {
	var mediaID *int
	if dbImage.MediaID.Valid {
		id := int(dbImage.MediaID.Int64)
		mediaID = &id
	}

	var width *int
	if dbImage.Width.Valid {
		w := int(dbImage.Width.Int64)
		width = &w
	}

	var height *int
	if dbImage.Height.Valid {
		h := int(dbImage.Height.Int64)
		height = &h
	}

	var fileSizeBytes *int64
	if dbImage.FileSizeBytes.Valid {
		fileSizeBytes = &dbImage.FileSizeBytes.Int64
	}

	var mimeType *string
	if dbImage.MimeType.Valid {
		mimeType = &dbImage.MimeType.String
	}

	var fileHash *string
	if dbImage.FileHash.Valid {
		fileHash = &dbImage.FileHash.String
	}

	var language *string
	if dbImage.Language.Valid {
		language = &dbImage.Language.String
	}

	var externalURL *string
	if dbImage.ExternalUrl.Valid {
		externalURL = &dbImage.ExternalUrl.String
	}

	var localCachePath *string
	if dbImage.LocalCachePath.Valid {
		localCachePath = &dbImage.LocalCachePath.String
	}

	priority := 0
	if dbImage.Priority.Valid {
		priority = int(dbImage.Priority.Int64)
	}

	filePath := ""
	if dbImage.FilePath.Valid {
		filePath = dbImage.FilePath.String
	}

	return &images.Image{
		ID:             int(dbImage.ID),
		MediaID:        mediaID,
		MediaType:      images.MediaType(dbImage.MediaType),
		EntityID:       int(dbImage.EntityID),
		ImageType:      images.ImageType(dbImage.ImageType),
		SourceType:     images.SourceType(dbImage.SourceType),
		FilePath:       filePath,
		ExternalURL:    externalURL,
		LocalCachePath: localCachePath,
		Width:          width,
		Height:         height,
		FileSizeBytes:  fileSizeBytes,
		MimeType:       mimeType,
		FileHash:       fileHash,
		Language:       language,
		Priority:       priority,
		CreatedAt:      dbImage.CreatedAt.Time,
		UpdatedAt:      dbImage.UpdatedAt.Time,
	}
}

// ========================================
// PostgreSQL Mappers
// ========================================

// postgresImageToDomain converts a PostgreSQL database image to a domain image
func postgresImageToDomain(dbImage sqlc_postgres.MediaImage) *images.Image {
	var mediaID *int
	if dbImage.MediaID.Valid {
		id := int(dbImage.MediaID.Int32)
		mediaID = &id
	}

	var width *int
	if dbImage.Width.Valid {
		w := int(dbImage.Width.Int32)
		width = &w
	}

	var height *int
	if dbImage.Height.Valid {
		h := int(dbImage.Height.Int32)
		height = &h
	}

	var fileSizeBytes *int64
	if dbImage.FileSizeBytes.Valid {
		fileSizeBytes = &dbImage.FileSizeBytes.Int64
	}

	var mimeType *string
	if dbImage.MimeType.Valid {
		mimeType = &dbImage.MimeType.String
	}

	var fileHash *string
	if dbImage.FileHash.Valid {
		fileHash = &dbImage.FileHash.String
	}

	var language *string
	if dbImage.Language.Valid {
		language = &dbImage.Language.String
	}

	var externalURL *string
	if dbImage.ExternalUrl.Valid {
		externalURL = &dbImage.ExternalUrl.String
	}

	var localCachePath *string
	if dbImage.LocalCachePath.Valid {
		localCachePath = &dbImage.LocalCachePath.String
	}

	priority := 0
	if dbImage.Priority.Valid {
		priority = int(dbImage.Priority.Int32)
	}

	filePath := ""
	if dbImage.FilePath.Valid {
		filePath = dbImage.FilePath.String
	}

	return &images.Image{
		ID:             int(dbImage.ID),
		MediaID:        mediaID,
		MediaType:      images.MediaType(dbImage.MediaType),
		EntityID:       int(dbImage.EntityID),
		ImageType:      images.ImageType(dbImage.ImageType),
		SourceType:     images.SourceType(dbImage.SourceType),
		FilePath:       filePath,
		ExternalURL:    externalURL,
		LocalCachePath: localCachePath,
		Width:          width,
		Height:         height,
		FileSizeBytes:  fileSizeBytes,
		MimeType:       mimeType,
		FileHash:       fileHash,
		Language:       language,
		Priority:       priority,
		CreatedAt:      dbImage.CreatedAt.Time,
		UpdatedAt:      dbImage.UpdatedAt.Time,
	}
}

// ========================================
// SQLite Param Builders
// ========================================

// buildSQLiteCreateImageParams converts a domain image to SQLite create params
func buildSQLiteCreateImageParams(image *images.Image) sqlc_sqlite.CreateImageParams {
	var mediaID sql.NullInt64
	if image.MediaID != nil {
		mediaID = sql.NullInt64{Int64: int64(*image.MediaID), Valid: true}
	}

	var width sql.NullInt64
	if image.Width != nil {
		width = sql.NullInt64{Int64: int64(*image.Width), Valid: true}
	}

	var height sql.NullInt64
	if image.Height != nil {
		height = sql.NullInt64{Int64: int64(*image.Height), Valid: true}
	}

	var fileSizeBytes sql.NullInt64
	if image.FileSizeBytes != nil {
		fileSizeBytes = sql.NullInt64{Int64: *image.FileSizeBytes, Valid: true}
	}

	var mimeType sql.NullString
	if image.MimeType != nil {
		mimeType = sql.NullString{String: *image.MimeType, Valid: true}
	}

	var fileHash sql.NullString
	if image.FileHash != nil {
		fileHash = sql.NullString{String: *image.FileHash, Valid: true}
	}

	var language sql.NullString
	if image.Language != nil {
		language = sql.NullString{String: *image.Language, Valid: true}
	}

	var externalURL sql.NullString
	if image.ExternalURL != nil {
		externalURL = sql.NullString{String: *image.ExternalURL, Valid: true}
	}

	var localCachePath sql.NullString
	if image.LocalCachePath != nil {
		localCachePath = sql.NullString{String: *image.LocalCachePath, Valid: true}
	}

	filePath := sql.NullString{}
	if image.FilePath != "" {
		filePath = sql.NullString{String: image.FilePath, Valid: true}
	}

	return sqlc_sqlite.CreateImageParams{
		MediaID:        mediaID,
		MediaType:      string(image.MediaType),
		EntityID:       int64(image.EntityID),
		ImageType:      string(image.ImageType),
		SourceType:     string(image.SourceType),
		FilePath:       filePath,
		ExternalUrl:    externalURL,
		LocalCachePath: localCachePath,
		Width:          width,
		Height:         height,
		FileSizeBytes:  fileSizeBytes,
		MimeType:       mimeType,
		FileHash:       fileHash,
		Language:       language,
		Priority:       sql.NullInt64{Int64: int64(image.Priority), Valid: true},
	}
}

// buildSQLiteUpdateImageParams converts a domain image to SQLite update params
func buildSQLiteUpdateImageParams(image *images.Image) sqlc_sqlite.UpdateImageParams {
	createParams := buildSQLiteCreateImageParams(image)

	return sqlc_sqlite.UpdateImageParams{
		MediaID:        createParams.MediaID,
		MediaType:      createParams.MediaType,
		EntityID:       createParams.EntityID,
		ImageType:      createParams.ImageType,
		SourceType:     createParams.SourceType,
		FilePath:       createParams.FilePath,
		ExternalUrl:    createParams.ExternalUrl,
		LocalCachePath: createParams.LocalCachePath,
		Width:          createParams.Width,
		Height:         createParams.Height,
		FileSizeBytes:  createParams.FileSizeBytes,
		MimeType:       createParams.MimeType,
		FileHash:       createParams.FileHash,
		Language:       createParams.Language,
		Priority:       createParams.Priority,
		ID:             int64(image.ID),
	}
}

// ========================================
// PostgreSQL Param Builders
// ========================================

// buildPostgresCreateImageParams converts a domain image to PostgreSQL create params
func buildPostgresCreateImageParams(image *images.Image) sqlc_postgres.CreateImageParams {
	var mediaID sql.NullInt32
	if image.MediaID != nil {
		mediaID = sql.NullInt32{Int32: int32(*image.MediaID), Valid: true}
	}

	var width sql.NullInt32
	if image.Width != nil {
		width = sql.NullInt32{Int32: int32(*image.Width), Valid: true}
	}

	var height sql.NullInt32
	if image.Height != nil {
		height = sql.NullInt32{Int32: int32(*image.Height), Valid: true}
	}

	var fileSizeBytes sql.NullInt64
	if image.FileSizeBytes != nil {
		fileSizeBytes = sql.NullInt64{Int64: *image.FileSizeBytes, Valid: true}
	}

	var mimeType sql.NullString
	if image.MimeType != nil {
		mimeType = sql.NullString{String: *image.MimeType, Valid: true}
	}

	var fileHash sql.NullString
	if image.FileHash != nil {
		fileHash = sql.NullString{String: *image.FileHash, Valid: true}
	}

	var language sql.NullString
	if image.Language != nil {
		language = sql.NullString{String: *image.Language, Valid: true}
	}

	var externalURL sql.NullString
	if image.ExternalURL != nil {
		externalURL = sql.NullString{String: *image.ExternalURL, Valid: true}
	}

	var localCachePath sql.NullString
	if image.LocalCachePath != nil {
		localCachePath = sql.NullString{String: *image.LocalCachePath, Valid: true}
	}

	filePath := sql.NullString{}
	if image.FilePath != "" {
		filePath = sql.NullString{String: image.FilePath, Valid: true}
	}

	return sqlc_postgres.CreateImageParams{
		MediaID:        mediaID,
		MediaType:      string(image.MediaType),
		EntityID:       int32(image.EntityID),
		ImageType:      string(image.ImageType),
		SourceType:     string(image.SourceType),
		FilePath:       filePath,
		ExternalUrl:    externalURL,
		LocalCachePath: localCachePath,
		Width:          width,
		Height:         height,
		FileSizeBytes:  fileSizeBytes,
		MimeType:       mimeType,
		FileHash:       fileHash,
		Language:       language,
		Priority:       sql.NullInt32{Int32: int32(image.Priority), Valid: true},
	}
}

// buildPostgresUpdateImageParams converts a domain image to PostgreSQL update params
func buildPostgresUpdateImageParams(image *images.Image) sqlc_postgres.UpdateImageParams {
	createParams := buildPostgresCreateImageParams(image)

	return sqlc_postgres.UpdateImageParams{
		MediaID:        createParams.MediaID,
		MediaType:      createParams.MediaType,
		EntityID:       createParams.EntityID,
		ImageType:      createParams.ImageType,
		SourceType:     createParams.SourceType,
		FilePath:       createParams.FilePath,
		ExternalUrl:    createParams.ExternalUrl,
		LocalCachePath: createParams.LocalCachePath,
		Width:          createParams.Width,
		Height:         createParams.Height,
		FileSizeBytes:  createParams.FileSizeBytes,
		MimeType:       createParams.MimeType,
		FileHash:       createParams.FileHash,
		Language:       createParams.Language,
		Priority:       createParams.Priority,
		ID:             int32(image.ID),
	}
}

// ========================================
// PostgreSQL Query Param Builders
// ========================================

func buildPostgresListImagesByEntityParams(mediaType images.MediaType, entityID int) sqlc_postgres.ListImagesByEntityParams {
	return sqlc_postgres.ListImagesByEntityParams{
		MediaType: string(mediaType),
		EntityID:  int32(entityID),
	}
}

func buildPostgresGetImageByTypeAndEntityParams(mediaType images.MediaType, entityID int, imageType images.ImageType) sqlc_postgres.GetImageByTypeAndEntityParams {
	return sqlc_postgres.GetImageByTypeAndEntityParams{
		MediaType: string(mediaType),
		EntityID:  int32(entityID),
		ImageType: string(imageType),
	}
}

func buildPostgresGetImageByTypeAndMediaIDParams(mediaID int, imageType images.ImageType) sqlc_postgres.GetImageByTypeAndMediaIDParams {
	return sqlc_postgres.GetImageByTypeAndMediaIDParams{
		MediaID:   common.NullInt32FromInt64(int64(mediaID)),
		ImageType: string(imageType),
	}
}

func buildPostgresDeleteImagesByEntityParams(mediaType images.MediaType, entityID int) sqlc_postgres.DeleteImagesByEntityParams {
	return sqlc_postgres.DeleteImagesByEntityParams{
		MediaType: string(mediaType),
		EntityID:  int32(entityID),
	}
}

// ========================================
// SQLite Query Param Builders
// ========================================

func buildSQLiteListImagesByEntityParams(mediaType images.MediaType, entityID int) sqlc_sqlite.ListImagesByEntityParams {
	return sqlc_sqlite.ListImagesByEntityParams{
		MediaType: string(mediaType),
		EntityID:  int64(entityID),
	}
}

func buildSQLiteGetImageByTypeAndEntityParams(mediaType images.MediaType, entityID int, imageType images.ImageType) sqlc_sqlite.GetImageByTypeAndEntityParams {
	return sqlc_sqlite.GetImageByTypeAndEntityParams{
		MediaType: string(mediaType),
		EntityID:  int64(entityID),
		ImageType: string(imageType),
	}
}

func buildSQLiteGetImageByTypeAndMediaIDParams(mediaID int, imageType images.ImageType) sqlc_sqlite.GetImageByTypeAndMediaIDParams {
	return sqlc_sqlite.GetImageByTypeAndMediaIDParams{
		MediaID:   common.NullInt64(int64(mediaID)),
		ImageType: string(imageType),
	}
}

func buildSQLiteDeleteImagesByEntityParams(mediaType images.MediaType, entityID int) sqlc_sqlite.DeleteImagesByEntityParams {
	return sqlc_sqlite.DeleteImagesByEntityParams{
		MediaType: string(mediaType),
		EntityID:  int64(entityID),
	}
}
