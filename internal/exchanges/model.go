package exchanges

type Exchange struct {
	ID          int    `json:"id"`
	ServiceID   int    `json:"service_id"`
	RequesterID int    `json:"requester_id"`
	OwnerID     int    `json:"owner_id"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

const (
	StatusPending   = "pending"
	StatusAccepted  = "accepted"
	StatusCompleted = "completed"
	StatusRejected  = "rejected"
	StatusCancelled = "cancelled"
)

func validStatus(status string) bool {
	switch status {
	case StatusPending, StatusAccepted, StatusCompleted, StatusRejected, StatusCancelled:
		return true
	default:
		return false
	}
}

type CreateRequest struct {
	ServiceID int `json:"service_id"`
}

type CreateParams struct {
	ServiceID   int
	RequesterID int
	OwnerID     int
	Cost        int
}

type Filter struct {
	UserID int
	Status string
}
