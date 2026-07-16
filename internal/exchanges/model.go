// Package exchanges owns the exchange lifecycle and drives the credit journal
// that goes with it: a requester asks for a service, the owner accepts or
// rejects, and an accepted exchange is completed or cancelled. Every status
// change that moves credits is executed atomically inside a single SQL
// transaction, so a status and its credit movement always commit or roll back
// together and no credit is ever transferred twice.
package exchanges

// Exchange is the public representation returned by the exchange endpoints.
// The credit cost blocked for the exchange is stored internally (see the
// credits_cost column) and is deliberately not exposed here.
type Exchange struct {
	ID          int    `json:"id"`
	ServiceID   int    `json:"service_id"`
	RequesterID int    `json:"requester_id"`
	OwnerID     int    `json:"owner_id"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Lifecycle statuses.
//
//	pending -> accepted -> completed
//	   |          |
//	   v          v
//	rejected   cancelled
//
// (a pending exchange can also be cancelled). Rejected, completed, and
// cancelled are terminal.
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

// CreateRequest is the POST /api/exchanges body. Only the service is supplied
// by the client; the owner and the price are resolved server-side from that
// service so a client cannot forge them.
type CreateRequest struct {
	ServiceID int `json:"service_id"`
}

// CreateParams carries the server-resolved values used to persist a request.
type CreateParams struct {
	ServiceID   int
	RequesterID int
	OwnerID     int
	Cost        int
}

// Filter selects the exchanges visible to one user, optionally narrowed to a
// single status.
type Filter struct {
	UserID int
	Status string
}
