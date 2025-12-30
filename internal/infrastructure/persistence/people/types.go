package people

import (
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// interfaceToString converts interface{} to string (used for PostgreSQL CHECK columns).
func interfaceToString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// --- Person Mappers ---

func sqlitePersonToDomain(p sqlc_sqlite.Person) *media.Person {
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

func postgresPersonToDomain(p sqlc_postgres.Person) *media.Person {
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

func sqliteCreditRowToDomain(row sqlc_sqlite.GetCreditsForEntityRow) *media.Credit {
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

func postgresCreditRowToDomain(row sqlc_postgres.GetCreditsForEntityRow) *media.Credit {
	return &media.Credit{
		ID:            row.ID,
		PersonID:      row.PersonID,
		MediaType:     interfaceToString(row.MediaType),
		EntityID:      row.EntityID,
		CreditType:    interfaceToString(row.CreditType),
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

func sqliteCastRowToDomain(row sqlc_sqlite.GetCastForEntityRow) *media.Credit {
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

func postgresCastRowToDomain(row sqlc_postgres.GetCastForEntityRow) *media.Credit {
	return &media.Credit{
		ID:            row.ID,
		PersonID:      row.PersonID,
		MediaType:     interfaceToString(row.MediaType),
		EntityID:      row.EntityID,
		CreditType:    interfaceToString(row.CreditType),
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

func sqliteDirectorRowToDomain(row sqlc_sqlite.GetDirectorsForEntityRow) *media.Credit {
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

func postgresDirectorRowToDomain(row sqlc_postgres.GetDirectorsForEntityRow) *media.Credit {
	return &media.Credit{
		ID:            row.ID,
		PersonID:      row.PersonID,
		MediaType:     interfaceToString(row.MediaType),
		EntityID:      row.EntityID,
		CreditType:    interfaceToString(row.CreditType),
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

func sqliteWriterRowToDomain(row sqlc_sqlite.GetWritersForEntityRow) *media.Credit {
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

func postgresWriterRowToDomain(row sqlc_postgres.GetWritersForEntityRow) *media.Credit {
	return &media.Credit{
		ID:            row.ID,
		PersonID:      row.PersonID,
		MediaType:     interfaceToString(row.MediaType),
		EntityID:      row.EntityID,
		CreditType:    interfaceToString(row.CreditType),
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

func sqliteCreatorRowToDomain(row sqlc_sqlite.GetCreatorsForEntityRow) *media.Credit {
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

func postgresCreatorRowToDomain(row sqlc_postgres.GetCreatorsForEntityRow) *media.Credit {
	return &media.Credit{
		ID:            row.ID,
		PersonID:      row.PersonID,
		MediaType:     interfaceToString(row.MediaType),
		EntityID:      row.EntityID,
		CreditType:    interfaceToString(row.CreditType),
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

func sqliteCreditsForPersonToDomain(row sqlc_sqlite.GetCreditsForPersonRow) *media.Credit {
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

func postgresCreditsForPersonToDomain(row sqlc_postgres.GetCreditsForPersonRow) *media.Credit {
	return &media.Credit{
		ID:            row.ID,
		PersonID:      row.PersonID,
		MediaType:     interfaceToString(row.MediaType),
		EntityID:      row.EntityID,
		CreditType:    interfaceToString(row.CreditType),
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

func buildSQLiteCreatePersonParams(p *media.Person) sqlc_sqlite.CreatePersonParams {
	return sqlc_sqlite.CreatePersonParams{
		Name:      p.Name,
		SortName:  common.NullString(p.SortName),
		PhotoPath: common.NullString(p.PhotoPath),
		PhotoUrl:  common.NullString(p.PhotoURL),
		ImdbID:    common.NullString(p.IMDbID),
		TmdbID:    common.NullInt64(int64(p.TMDbID)),
	}
}

func buildPostgresCreatePersonParams(p *media.Person) sqlc_postgres.CreatePersonParams {
	return sqlc_postgres.CreatePersonParams{
		Name:      p.Name,
		SortName:  common.NullString(p.SortName),
		PhotoPath: common.NullString(p.PhotoPath),
		PhotoUrl:  common.NullString(p.PhotoURL),
		ImdbID:    common.NullString(p.IMDbID),
		TmdbID:    common.NullInt64(int64(p.TMDbID)),
	}
}

func buildSQLiteUpdatePersonParams(p *media.Person) sqlc_sqlite.UpdatePersonParams {
	return sqlc_sqlite.UpdatePersonParams{
		ID:        p.ID,
		Name:      p.Name,
		SortName:  common.NullString(p.SortName),
		PhotoPath: common.NullString(p.PhotoPath),
		PhotoUrl:  common.NullString(p.PhotoURL),
		ImdbID:    common.NullString(p.IMDbID),
		TmdbID:    common.NullInt64(int64(p.TMDbID)),
	}
}

func buildPostgresUpdatePersonParams(p *media.Person) sqlc_postgres.UpdatePersonParams {
	return sqlc_postgres.UpdatePersonParams{
		ID:        p.ID,
		Name:      p.Name,
		SortName:  common.NullString(p.SortName),
		PhotoPath: common.NullString(p.PhotoPath),
		PhotoUrl:  common.NullString(p.PhotoURL),
		ImdbID:    common.NullString(p.IMDbID),
		TmdbID:    common.NullInt64(int64(p.TMDbID)),
	}
}

func buildSQLiteCreateCreditParams(c *media.Credit) sqlc_sqlite.CreateCreditParams {
	return sqlc_sqlite.CreateCreditParams{
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

func buildPostgresCreateCreditParams(c *media.Credit) sqlc_postgres.CreateCreditParams {
	return sqlc_postgres.CreateCreditParams{
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
