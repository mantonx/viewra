package people

import "github.com/mantonx/viewra/internal/domain/media"

// PersonResponse represents a person in API responses.
type PersonResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortName  string `json:"sort_name,omitempty"`
	PhotoPath string `json:"photo_path,omitempty"`
	PhotoURL  string `json:"photo_url,omitempty"`
	IMDbID    string `json:"imdb_id,omitempty"`
	TMDbID    int    `json:"tmdb_id,omitempty"`
}

// CreditResponse represents a credit in API responses.
type CreditResponse struct {
	ID            int64           `json:"id"`
	Person        *PersonResponse `json:"person,omitempty"`
	CreditType    string          `json:"credit_type"`
	CharacterName string          `json:"character_name,omitempty"`
	Department    string          `json:"department,omitempty"`
	Job           string          `json:"job,omitempty"`
	BillingOrder  int             `json:"billing_order"`
}

// CreditsResponse contains credits grouped by type.
type CreditsResponse struct {
	Cast      []CreditResponse `json:"cast"`
	Directors []CreditResponse `json:"directors"`
	Writers   []CreditResponse `json:"writers"`
	Creators  []CreditResponse `json:"creators,omitempty"`
}

// PersonCreditsResponse contains a person's filmography.
type PersonCreditsResponse struct {
	Person  PersonResponse   `json:"person"`
	Credits []CreditResponse `json:"credits"`
}

// ToPersonResponse converts a domain Person to a response DTO.
func ToPersonResponse(p *media.Person) PersonResponse {
	return PersonResponse{
		ID:        p.ID,
		Name:      p.Name,
		SortName:  p.SortName,
		PhotoPath: p.PhotoPath,
		PhotoURL:  p.PhotoURL,
		IMDbID:    p.IMDbID,
		TMDbID:    p.TMDbID,
	}
}

// ToCreditResponse converts a domain Credit to a response DTO.
func ToCreditResponse(c *media.Credit) CreditResponse {
	resp := CreditResponse{
		ID:            c.ID,
		CreditType:    c.CreditType,
		CharacterName: c.CharacterName,
		Department:    c.Department,
		Job:           c.Job,
		BillingOrder:  c.BillingOrder,
	}
	if c.Person != nil {
		personResp := ToPersonResponse(c.Person)
		resp.Person = &personResp
	}
	return resp
}

// ToCreditsResponse converts a list of domain Credits to a grouped response.
func ToCreditsResponse(credits []*media.Credit) CreditsResponse {
	resp := CreditsResponse{
		Cast:      []CreditResponse{},
		Directors: []CreditResponse{},
		Writers:   []CreditResponse{},
		Creators:  []CreditResponse{},
	}

	for _, c := range credits {
		creditResp := ToCreditResponse(c)
		switch c.CreditType {
		case media.CreditTypeCast:
			resp.Cast = append(resp.Cast, creditResp)
		case media.CreditTypeDirector:
			resp.Directors = append(resp.Directors, creditResp)
		case media.CreditTypeWriter:
			resp.Writers = append(resp.Writers, creditResp)
		case media.CreditTypeCreator:
			resp.Creators = append(resp.Creators, creditResp)
		}
	}

	return resp
}
