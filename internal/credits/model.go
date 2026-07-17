package credits

type CreditTransaction struct {
	ID         int    `json:"id"`
	UserID     int    `json:"user_id"`
	ExchangeID int    `json:"exchange_id"`
	Montant    int    `json:"montant"`
	Type       string `json:"type"`
	CreatedAt  string `json:"created_at"`
}

const (
	TypeEarn   = "earn"
	TypeSpend  = "spend"
	TypeRefund = "refund"
)

type Entry struct {
	UserID     int
	ExchangeID int
	Amount     int
	Type       string
}
