package people

import (
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// --- Person Mapper ---

func personToDomain(p unified.Person) *media.Person {
	return &media.Person{
		ID:        p.ID,
		Name:      p.Name,
		SortName:  common.ParseNullString(p.SortName),
		PhotoPath: common.ParseNullString(p.PhotoPath),
		PhotoURL:  common.ParseNullString(p.PhotoUrl),
		IMDbID:    common.ParseNullString(p.ImdbID),
		TMDbID:    int(common.ParseNullInt64(p.TmdbID)),
	}
}

// --- Credit Row Mappers (with joined person data) ---

func creditRowToDomain(row unified.GetCreditsForEntityRow) *media.Credit {
	return &media.Credit{
		ID:            row.ID,
		PersonID:      row.PersonID,
		MediaType:     row.MediaType,
		EntityID:      row.EntityID,
		CreditType:    row.CreditType,
		CharacterName: common.ParseNullString(row.CharacterName),
		Department:    common.ParseNullString(row.Department),
		Job:           common.ParseNullString(row.Job),
		BillingOrder:  int(common.ParseNullInt64(row.BillingOrder)),
		Person: &media.Person{
			ID:        row.PersonID_2,
			Name:      row.PersonName,
			SortName:  common.ParseNullString(row.PersonSortName),
			PhotoPath: common.ParseNullString(row.PersonPhotoPath),
			PhotoURL:  common.ParseNullString(row.PersonPhotoUrl),
			IMDbID:    common.ParseNullString(row.PersonImdbID),
			TMDbID:    int(common.ParseNullInt64(row.PersonTmdbID)),
		},
	}
}

func castRowToDomain(row unified.GetCastForEntityRow) *media.Credit {
	return &media.Credit{
		ID:            row.ID,
		PersonID:      row.PersonID,
		MediaType:     row.MediaType,
		EntityID:      row.EntityID,
		CreditType:    row.CreditType,
		CharacterName: common.ParseNullString(row.CharacterName),
		Department:    common.ParseNullString(row.Department),
		Job:           common.ParseNullString(row.Job),
		BillingOrder:  int(common.ParseNullInt64(row.BillingOrder)),
		Person: &media.Person{
			ID:        row.PersonID_2,
			Name:      row.PersonName,
			SortName:  common.ParseNullString(row.PersonSortName),
			PhotoPath: common.ParseNullString(row.PersonPhotoPath),
			PhotoURL:  common.ParseNullString(row.PersonPhotoUrl),
			IMDbID:    common.ParseNullString(row.PersonImdbID),
			TMDbID:    int(common.ParseNullInt64(row.PersonTmdbID)),
		},
	}
}

func directorRowToDomain(row unified.GetDirectorsForEntityRow) *media.Credit {
	return &media.Credit{
		ID:            row.ID,
		PersonID:      row.PersonID,
		MediaType:     row.MediaType,
		EntityID:      row.EntityID,
		CreditType:    row.CreditType,
		CharacterName: common.ParseNullString(row.CharacterName),
		Department:    common.ParseNullString(row.Department),
		Job:           common.ParseNullString(row.Job),
		BillingOrder:  int(common.ParseNullInt64(row.BillingOrder)),
		Person: &media.Person{
			ID:        row.PersonID_2,
			Name:      row.PersonName,
			SortName:  common.ParseNullString(row.PersonSortName),
			PhotoPath: common.ParseNullString(row.PersonPhotoPath),
			PhotoURL:  common.ParseNullString(row.PersonPhotoUrl),
			IMDbID:    common.ParseNullString(row.PersonImdbID),
			TMDbID:    int(common.ParseNullInt64(row.PersonTmdbID)),
		},
	}
}

func writerRowToDomain(row unified.GetWritersForEntityRow) *media.Credit {
	return &media.Credit{
		ID:            row.ID,
		PersonID:      row.PersonID,
		MediaType:     row.MediaType,
		EntityID:      row.EntityID,
		CreditType:    row.CreditType,
		CharacterName: common.ParseNullString(row.CharacterName),
		Department:    common.ParseNullString(row.Department),
		Job:           common.ParseNullString(row.Job),
		BillingOrder:  int(common.ParseNullInt64(row.BillingOrder)),
		Person: &media.Person{
			ID:        row.PersonID_2,
			Name:      row.PersonName,
			SortName:  common.ParseNullString(row.PersonSortName),
			PhotoPath: common.ParseNullString(row.PersonPhotoPath),
			PhotoURL:  common.ParseNullString(row.PersonPhotoUrl),
			IMDbID:    common.ParseNullString(row.PersonImdbID),
			TMDbID:    int(common.ParseNullInt64(row.PersonTmdbID)),
		},
	}
}

func creatorRowToDomain(row unified.GetCreatorsForEntityRow) *media.Credit {
	return &media.Credit{
		ID:            row.ID,
		PersonID:      row.PersonID,
		MediaType:     row.MediaType,
		EntityID:      row.EntityID,
		CreditType:    row.CreditType,
		CharacterName: common.ParseNullString(row.CharacterName),
		Department:    common.ParseNullString(row.Department),
		Job:           common.ParseNullString(row.Job),
		BillingOrder:  int(common.ParseNullInt64(row.BillingOrder)),
		Person: &media.Person{
			ID:        row.PersonID_2,
			Name:      row.PersonName,
			SortName:  common.ParseNullString(row.PersonSortName),
			PhotoPath: common.ParseNullString(row.PersonPhotoPath),
			PhotoURL:  common.ParseNullString(row.PersonPhotoUrl),
			IMDbID:    common.ParseNullString(row.PersonImdbID),
			TMDbID:    int(common.ParseNullInt64(row.PersonTmdbID)),
		},
	}
}

func creditsForPersonToDomain(row unified.GetCreditsForPersonRow) *media.Credit {
	return &media.Credit{
		ID:            row.ID,
		PersonID:      row.PersonID,
		MediaType:     row.MediaType,
		EntityID:      row.EntityID,
		CreditType:    row.CreditType,
		CharacterName: common.ParseNullString(row.CharacterName),
		Department:    common.ParseNullString(row.Department),
		Job:           common.ParseNullString(row.Job),
		BillingOrder:  int(common.ParseNullInt64(row.BillingOrder)),
		Person: &media.Person{
			ID:        row.PersonID_2,
			Name:      row.PersonName,
			SortName:  common.ParseNullString(row.PersonSortName),
			PhotoPath: common.ParseNullString(row.PersonPhotoPath),
			PhotoURL:  common.ParseNullString(row.PersonPhotoUrl),
			IMDbID:    common.ParseNullString(row.PersonImdbID),
			TMDbID:    int(common.ParseNullInt64(row.PersonTmdbID)),
		},
	}
}

// --- Param Builders ---

func buildCreatePersonParams(p *media.Person) unified.CreatePersonParams {
	return unified.CreatePersonParams{
		Name:      p.Name,
		SortName:  common.NullString(p.SortName),
		PhotoPath: common.NullString(p.PhotoPath),
		PhotoUrl:  common.NullString(p.PhotoURL),
		ImdbID:    common.NullString(p.IMDbID),
		TmdbID:    common.NullInt64(int64(p.TMDbID)),
	}
}

func buildUpdatePersonParams(p *media.Person) unified.UpdatePersonParams {
	return unified.UpdatePersonParams{
		ID:        p.ID,
		Name:      p.Name,
		SortName:  common.NullString(p.SortName),
		PhotoPath: common.NullString(p.PhotoPath),
		PhotoUrl:  common.NullString(p.PhotoURL),
		ImdbID:    common.NullString(p.IMDbID),
		TmdbID:    common.NullInt64(int64(p.TMDbID)),
	}
}

func buildCreateCreditParams(c *media.Credit) unified.CreateCreditParams {
	return unified.CreateCreditParams{
		PersonID:      c.PersonID,
		MediaType:     c.MediaType,
		EntityID:      c.EntityID,
		CreditType:    c.CreditType,
		CharacterName: common.NullString(c.CharacterName),
		Department:    common.NullString(c.Department),
		Job:           common.NullString(c.Job),
		BillingOrder:  sql.NullInt64{Int64: int64(c.BillingOrder), Valid: true},
	}
}
