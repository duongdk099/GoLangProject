package reviews

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

type CreateParams struct {
	ExchangeID  int
	ServiceID   int
	AuthorID    int
	TargetID    int
	Note        int
	Commentaire string
}

type ExchangeSummary struct {
	ID          int
	ServiceID   int
	RequesterID int
	OwnerID     int
	Status      string
}

const StatusCompleted = "completed"
