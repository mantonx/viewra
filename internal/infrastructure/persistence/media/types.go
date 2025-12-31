package media

import (
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/quality"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements media.Repository using sqlc.
type Repository struct {
	*common.BaseRepository
}

// calculateQualityScore computes a quality score for a media entity using technical characteristics.
func calculateQualityScore(m *media.Media) int {
	return quality.Calculate(quality.MediaParams{
		Width:      m.Width,
		Height:     m.Height,
		VideoCodec: m.VideoCodec,
		AudioCodec: m.AudioCodec,
		Bitrate:    m.Bitrate,
		HDRFormat:  m.HDRFormat,
	})
}

// mediumToDomain converts a database Medium model to a domain Media entity.
func mediumToDomain(sq unified.Medium) *media.Media {
	return &media.Media{
		ID:              sq.ID,
		LibraryID:       sq.LibraryID,
		Title:           sq.Title,
		Type:            sq.Type,
		FilePath:        sq.FilePath,
		FileSize:        sq.FileSize.Int64,
		FileHash:        sq.FileHash.String,
		Duration:        int(sq.Duration.Float64),
		IsExtra:         common.Int64ToBool(sq.IsExtra),
		Width:           int(sq.Width.Int64),
		Height:          int(sq.Height.Int64),
		VideoCodec:      sq.Codec.String,
		AudioCodec:      sq.AudioCodec.String,
		Bitrate:         sq.BitRate.Int64,
		FrameRate:       sq.FrameRate.Float64,
		ContainerFormat: sq.ContainerFormat.String,
		CodecProfile:    sq.CodecProfile.String,
		ScanType:        sq.ScanType.String,
		HDRFormat:       sq.HdrFormat.String,
		ColorSpace:      sq.ColorSpace.String,
		ColorPrimaries:  sq.ColorPrimaries.String,
		CreatedAt:       common.ParseNullTime(sq.CreatedAt),
		UpdatedAt:       common.ParseNullTime(sq.UpdatedAt),
	}
}

// buildCreateParams builds CreateMediaParams from a domain Media entity.
func buildCreateParams(m *media.Media) unified.CreateMediaParams {
	return unified.CreateMediaParams{
		LibraryID:         m.LibraryID,
		Title:             m.Title,
		FilePath:          m.FilePath,
		FileSize:          common.NullInt64(m.FileSize),
		Duration:          common.NullFloat64(float64(m.Duration)),
		Type:              m.Type,
		IsExtra:           common.BoolToInt64(m.IsExtra),
		FileHash:          common.NullString(m.FileHash),
		ContainerFormat:   common.NullString(m.ContainerFormat),
		Width:             common.NullInt64(int64(m.Width)),
		Height:            common.NullInt64(int64(m.Height)),
		AspectRatio:       common.NullString(media.CalculateAspectRatio(m.Width, m.Height)),
		Codec:             common.NullString(m.VideoCodec),
		AudioCodec:        common.NullString(m.AudioCodec),
		CodecProfile:      common.NullString(m.CodecProfile),
		BitRate:           common.NullInt64(m.Bitrate),
		FrameRate:         common.NullFloat64(m.FrameRate),
		ScanType:          common.NullString(m.ScanType),
		HdrFormat:         common.NullString(m.HDRFormat),
		ColorSpace:        common.NullString(m.ColorSpace),
		ColorPrimaries:    common.NullString(m.ColorPrimaries),
		ThumbnailPath:     sql.NullString{},
		SourceType:        common.NullString(media.DetectSourceType(m.FilePath)),
		ResolutionLabel:   common.NullString(media.CalculateResolutionLabelFromDimensions(m.Width, m.Height)),
		QualityScore:      common.NullInt64(int64(calculateQualityScore(m))),
		Is3d:              common.NullInt64FromBool(func() bool { is3d, _ := media.Detect3D(m.FilePath); return is3d }()),
		StereoMode:        common.NullString(func() string { _, stereoMode := media.Detect3D(m.FilePath); return stereoMode }()),
		HasDash:           common.NullInt64FromBool(false),
		DashManifestPath:  sql.NullString{},
		TranscodingStatus: sql.NullString{},
	}
}

// buildUpdateParams builds UpdateMediaParams from a domain Media entity.
func buildUpdateParams(m *media.Media) unified.UpdateMediaParams {
	params := buildCreateParams(m)
	return unified.UpdateMediaParams{
		LibraryID:         params.LibraryID,
		Title:             params.Title,
		FilePath:          params.FilePath,
		FileSize:          params.FileSize,
		Duration:          params.Duration,
		Type:              params.Type,
		IsExtra:           params.IsExtra,
		FileHash:          params.FileHash,
		ContainerFormat:   params.ContainerFormat,
		Width:             params.Width,
		Height:            params.Height,
		AspectRatio:       params.AspectRatio,
		Codec:             params.Codec,
		AudioCodec:        params.AudioCodec,
		CodecProfile:      params.CodecProfile,
		BitRate:           params.BitRate,
		FrameRate:         params.FrameRate,
		ScanType:          params.ScanType,
		HdrFormat:         params.HdrFormat,
		ColorSpace:        params.ColorSpace,
		ColorPrimaries:    params.ColorPrimaries,
		ThumbnailPath:     params.ThumbnailPath,
		SourceType:        params.SourceType,
		ResolutionLabel:   params.ResolutionLabel,
		QualityScore:      params.QualityScore,
		Is3d:              params.Is3d,
		StereoMode:        params.StereoMode,
		HasDash:           params.HasDash,
		DashManifestPath:  params.DashManifestPath,
		TranscodingStatus: params.TranscodingStatus,
		ID:                m.ID,
	}
}
