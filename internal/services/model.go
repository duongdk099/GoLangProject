// Package services owns service advertisements: publishing, updating,
// deleting, and server-side filtered listing.
package services

type Service struct {
	ID           int    `json:"id"`
	ProviderID   int    `json:"provider_id"`
	Titre        string `json:"titre"`
	Description  string `json:"description,omitempty"`
	Categorie    string `json:"categorie"`
	DureeMinutes int    `json:"duree_minutes"`
	Credits      int    `json:"credits"`
	Ville        string `json:"ville,omitempty"`
	Actif        bool   `json:"actif"`
	CreatedAt    string `json:"created_at"`
}

// categories is the closed list of allowed categories from the subject. It
// intentionally spells "Demenagement" without an accent to keep the value
// stable across source encodings.
var categories = map[string]struct{}{
	"Informatique": {}, "Jardinage": {}, "Bricolage": {}, "Cuisine": {},
	"Musique": {}, "Langues": {}, "Sport": {}, "Tutorat": {},
	"Demenagement": {}, "Photographie": {}, "Animalier": {}, "Couture": {},
	"Autre": {},
}

func validCategory(categorie string) bool {
	_, ok := categories[categorie]
	return ok
}

type CreateRequest struct {
	Titre        string `json:"titre"`
	Description  string `json:"description"`
	Categorie    string `json:"categorie"`
	DureeMinutes int    `json:"duree_minutes"`
	Credits      int    `json:"credits"`
	Ville        string `json:"ville"`
}

type UpdateRequest struct {
	Titre        string `json:"titre"`
	Description  string `json:"description"`
	Categorie    string `json:"categorie"`
	DureeMinutes int    `json:"duree_minutes"`
	Credits      int    `json:"credits"`
	Ville        string `json:"ville"`
	Actif        bool   `json:"actif"`
}

type CreateParams struct {
	ProviderID   int
	Titre        string
	Description  string
	Categorie    string
	DureeMinutes int
	Credits      int
	Ville        string
}

type UpdateParams struct {
	Titre        string
	Description  string
	Categorie    string
	DureeMinutes int
	Credits      int
	Ville        string
	Actif        bool
}

// Filter carries the server-side filters accepted by GET /api/services.
// Every non-empty field narrows the result set.
type Filter struct {
	Categorie string
	Ville     string
	Search    string
}
