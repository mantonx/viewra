package image

import (
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// imageToDomain converts a database image to a domain image (unified for both SQLite and PostgreSQL)
func imageToDomain(dbImage unified.MediaImage) *images.Image {
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

// buildCreateImageParams converts a domain image to create params
func buildCreateImageParams(image *images.Image) unified.CreateImageParams {
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

	return unified.CreateImageParams{
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

// buildUpdateImageParams converts a domain image to update params
func buildUpdateImageParams(image *images.Image) unified.UpdateImageParams {
	createParams := buildCreateImageParams(image)

	return unified.UpdateImageParams{
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

// buildListImagesByEntityParams builds params for listing images by entity
func buildListImagesByEntityParams(mediaType images.MediaType, entityID int) unified.ListImagesByEntityParams {
	return unified.ListImagesByEntityParams{
		MediaType: string(mediaType),
		EntityID:  int64(entityID),
	}
}

// buildGetImageByTypeAndEntityParams builds params for getting image by type and entity
func buildGetImageByTypeAndEntityParams(mediaType images.MediaType, entityID int, imageType images.ImageType) unified.GetImageByTypeAndEntityParams {
	return unified.GetImageByTypeAndEntityParams{
		MediaType: string(mediaType),
		EntityID:  int64(entityID),
		ImageType: string(imageType),
	}
}

// buildGetImageByTypeAndMediaIDParams builds params for getting image by type and media ID
func buildGetImageByTypeAndMediaIDParams(mediaID int, imageType images.ImageType) unified.GetImageByTypeAndMediaIDParams {
	return unified.GetImageByTypeAndMediaIDParams{
		MediaID:   common.NullInt64(int64(mediaID)),
		ImageType: string(imageType),
	}
}

// buildDeleteImagesByEntityParams builds params for deleting images by entity
func buildDeleteImagesByEntityParams(mediaType images.MediaType, entityID int) unified.DeleteImagesByEntityParams {
	return unified.DeleteImagesByEntityParams{
		MediaType: string(mediaType),
		EntityID:  int64(entityID),
	}
}
