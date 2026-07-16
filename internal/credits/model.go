// Package credits owns the append-only credit journal. A user's balance is
// the sum of their entries rather than a mutable column, so every credit
// movement is traceable and no update can silently overwrite a balance.
//
// The journal is deliberately not a feature package with its own HTTP surface:
// it is a small library that other features (users' welcome credits, the
// exchange lifecycle) call to record movements, often inside a transaction
// they already own so the movement commits or rolls back together with the
// status change that caused it.
package credits

// CreditTransaction is one movement in the journal. A positive Montant credits
// the user (earn, refund); a negative Montant debits them (spend). ExchangeID
// is 0 for the welcome credit, which is not tied to any exchange.
type CreditTransaction struct {
	ID         int    `json:"id"`
	UserID     int    `json:"user_id"`
	ExchangeID int    `json:"exchange_id"`
	Montant    int    `json:"montant"`
	Type       string `json:"type"`
	CreatedAt  string `json:"created_at"`
}

// Movement types stored in credit_transactions.type.
const (
	TypeEarn   = "earn"   // credited to a user (welcome credit, service earning)
	TypeSpend  = "spend"  // debited from a requester when an exchange is accepted
	TypeRefund = "refund" // returned to a requester when an accepted exchange is cancelled
)

// Entry describes a movement to record. Amount is always a positive magnitude;
// the stored sign is derived from Type. ExchangeID is 0 when the movement is
// not linked to an exchange (the welcome credit).
type Entry struct {
	UserID     int
	ExchangeID int
	Amount     int
	Type       string
}
