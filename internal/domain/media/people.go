package media

// Person represents a person (actor, director, writer, etc.) in the media database.
type Person struct {
	ID        int64
	Name      string
	SortName  string
	PhotoPath string
	PhotoURL  string
	IMDbID    string
	TMDbID    int
}

// Credit represents a person's role in a media item.
type Credit struct {
	ID            int64
	PersonID      int64
	Person        *Person // Populated on fetch
	MediaType     string  // "movie", "tv_show", "tv_episode"
	EntityID      int64
	CreditType    string // "cast", "director", "writer", "creator"
	CharacterName string
	Department    string
	Job           string
	BillingOrder  int
}

// CreditType constants for credit types.
const (
	CreditTypeCast     = "cast"
	CreditTypeDirector = "director"
	CreditTypeWriter   = "writer"
	CreditTypeCreator  = "creator"
)

// PeopleRepository defines the interface for people and credits persistence.
type PeopleRepository interface {
	// Person operations
	GetPersonByID(id int64) (*Person, error)
	GetPersonByName(name string) (*Person, error)
	GetPersonByTMDbID(tmdbID int) (*Person, error)
	CreatePerson(person *Person) error
	UpdatePerson(person *Person) error
	FindOrCreatePerson(name string, tmdbID int) (*Person, error)
	// FindOrCreatePersonWithPhoto finds or creates a person, also setting/updating their photo URL.
	FindOrCreatePersonWithPhoto(name string, tmdbID int, photoURL string) (*Person, error)

	// Credit operations
	GetCreditsForEntity(mediaType string, entityID int64) ([]*Credit, error)
	GetCreditsForPerson(personID int64) ([]*Credit, error)
	CreateCredit(credit *Credit) error
	DeleteCreditsForEntity(mediaType string, entityID int64) error
	ReplaceCreditsForEntity(mediaType string, entityID int64, credits []*Credit) error
}
