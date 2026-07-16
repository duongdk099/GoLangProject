// Package reviews owns the ratings users leave on the other participant of
// a completed exchange.
package reviews

// Review is the public rating a user leaves on the other participant of a
// completed exchange.
type Review struct {
	ID          int    `json:"id"`
	ExchangeID  int    `json:"exchange_id"`
	AuthorID    int    `json:"author_id"`
	TargetID    int    `json:"target_id"`
	Note        int    `json:"note"`
	Commentaire string `json:"commentaire,omitempty"`
	CreatedAt   string `json:"created_at"`
}

type CreateRequest struct {
	Note        int    `json:"note"`
	Commentaire string `json:"commentaire"`
}

// CreateParams carries the values needed to persist a review. ServiceID is a
// denormalized snapshot resolved from the exchange at creation time; it is
// stored for GET /api/services/{id}/reviews but is not part of the public
// Review JSON representation.
type CreateParams struct {
	ExchangeID  int
	ServiceID   int
	AuthorID    int
	TargetID    int
	Note        int
	Commentaire string
}

// ExchangeSummary is the minimal exchange information review validation
// needs. It is produced by whichever store implements ExchangeLookup.
type ExchangeSummary struct {
	ID          int
	ServiceID   int
	RequesterID int
	OwnerID     int
	Status      string
}

const StatusCompleted = "completed"
