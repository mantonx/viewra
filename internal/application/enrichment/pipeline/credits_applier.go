package pipeline

import (
	"context"
	"log/slog"
	"strings"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/domain/media"
)

// CreditsApplier handles applying credits (cast, directors, writers, creators) from enrichment.
type CreditsApplier struct {
	peopleRepo media.PeopleRepository
	logger     *slog.Logger
}

// NewCreditsApplier creates a new CreditsApplier.
func NewCreditsApplier(peopleRepo media.PeopleRepository, logger *slog.Logger) *CreditsApplier {
	return &CreditsApplier{
		peopleRepo: peopleRepo,
		logger:     logger,
	}
}

// Apply applies credits from enriched metadata to the database.
func (a *CreditsApplier) Apply(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, metadata *pluginv1.EnrichedMetadata) error {
	if a.peopleRepo == nil {
		return nil // No people repo configured
	}

	// Determine the entity type for credits storage
	entityType := mediaTypeToEntityType(mediaType)
	if entityType == "" {
		return nil // Not a supported type for credits
	}

	// Check if we have any credits to apply
	hasCast := len(metadata.Cast) > 0
	hasDirectors := len(metadata.Directors) > 0
	hasWriters := len(metadata.Writers) > 0
	hasCreators := len(metadata.Creators) > 0
	hasProducers := len(metadata.Producers) > 0
	if !hasCast && !hasDirectors && !hasWriters && !hasCreators && !hasProducers {
		return nil
	}

	// IMPORTANT: Delete existing credits FIRST before finding/creating persons.
	// The orphan cleanup trigger (tr_credits_cleanup_orphan_people) deletes persons
	// who have no remaining credits. If we find/create persons before deleting,
	// those person IDs may become invalid after deletion.
	if err := a.peopleRepo.DeleteCreditsForEntity(entityType, mediaID); err != nil {
		return err
	}

	// Now find/create persons and add new credits
	var credits []*media.Credit

	// Process cast members
	for i, castMember := range metadata.Cast {
		// Cast members have TMDb ID and photo URL, use the extended method
		person, err := a.peopleRepo.FindOrCreatePersonWithPhoto(castMember.Name, int(castMember.TmdbId), castMember.Thumb)
		if err != nil {
			a.logger.Warn("failed to find/create person for cast",
				slog.String("name", castMember.Name),
				slog.Any("error", err))
			continue
		}

		credits = append(credits, &media.Credit{
			PersonID:      person.ID,
			Person:        person,
			MediaType:     entityType,
			EntityID:      mediaID,
			CreditType:    media.CreditTypeCast,
			CharacterName: castMember.Role,
			BillingOrder:  int(castMember.Order),
		})

		// Only store top N cast to avoid bloat
		if i >= 50 {
			break
		}
	}

	// Process directors
	for i, directorName := range metadata.Directors {
		person, err := a.findOrCreatePerson(directorName, 0)
		if err != nil {
			a.logger.Warn("failed to find/create person for director",
				slog.String("name", directorName),
				slog.Any("error", err))
			continue
		}

		credits = append(credits, &media.Credit{
			PersonID:     person.ID,
			Person:       person,
			MediaType:    entityType,
			EntityID:     mediaID,
			CreditType:   media.CreditTypeDirector,
			Department:   "Directing",
			Job:          "Director",
			BillingOrder: i,
		})
	}

	// Process writers
	for i, writerName := range metadata.Writers {
		person, err := a.findOrCreatePerson(writerName, 0)
		if err != nil {
			a.logger.Warn("failed to find/create person for writer",
				slog.String("name", writerName),
				slog.Any("error", err))
			continue
		}

		credits = append(credits, &media.Credit{
			PersonID:     person.ID,
			Person:       person,
			MediaType:    entityType,
			EntityID:     mediaID,
			CreditType:   media.CreditTypeWriter,
			Department:   "Writing",
			Job:          "Writer",
			BillingOrder: i,
		})
	}

	// Process creators (TV shows)
	for i, creatorName := range metadata.Creators {
		person, err := a.findOrCreatePerson(creatorName, 0)
		if err != nil {
			a.logger.Warn("failed to find/create person for creator",
				slog.String("name", creatorName),
				slog.Any("error", err))
			continue
		}

		credits = append(credits, &media.Credit{
			PersonID:     person.ID,
			Person:       person,
			MediaType:    entityType,
			EntityID:     mediaID,
			CreditType:   media.CreditTypeCreator,
			Department:   "Production",
			Job:          "Creator",
			BillingOrder: i,
		})
	}

	// Process producers
	for i, producerName := range metadata.Producers {
		person, err := a.findOrCreatePerson(producerName, 0)
		if err != nil {
			a.logger.Warn("failed to find/create person for producer",
				slog.String("name", producerName),
				slog.Any("error", err))
			continue
		}

		credits = append(credits, &media.Credit{
			PersonID:     person.ID,
			Person:       person,
			MediaType:    entityType,
			EntityID:     mediaID,
			CreditType:   media.CreditTypeProducer,
			Department:   "Production",
			Job:          "Producer",
			BillingOrder: i,
		})
	}

	// Create new credits
	for _, credit := range credits {
		if err := a.peopleRepo.CreateCredit(credit); err != nil {
			// Ignore FK errors - can happen due to race conditions with orphan cleanup trigger
			if !strings.Contains(err.Error(), "FOREIGN KEY constraint") {
				a.logger.Warn("failed to create credit",
					slog.String("person", credit.Person.Name),
					slog.Any("error", err))
			}
		}
	}

	a.logger.Debug("applied credits",
		slog.Int64("media_id", mediaID),
		slog.String("media_type", string(mediaType)),
		slog.Int("credit_count", len(credits)))

	return nil
}

// findOrCreatePerson finds or creates a person by name.
func (a *CreditsApplier) findOrCreatePerson(name string, tmdbID int) (*media.Person, error) {
	return a.peopleRepo.FindOrCreatePerson(name, tmdbID)
}

// mediaTypeToEntityType converts enrichment.MediaType to credits entity type.
func mediaTypeToEntityType(mt enrichment.MediaType) string {
	switch mt {
	case enrichment.MediaTypeMovie:
		return "movie"
	case enrichment.MediaTypeTVShow:
		return "tv_show"
	case enrichment.MediaTypeTV:
		return "tv_episode"
	default:
		return ""
	}
}
