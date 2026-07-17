package credits

import (
	"fmt"

	"barterswap/pkg/httpapi"
)

func Validate(entry Entry) error {
	if entry.UserID <= 0 {
		return fmt.Errorf("%w: credit entry requires a user", httpapi.ErrValidation)
	}
	if entry.Amount <= 0 {
		return fmt.Errorf("%w: credit amount must be positive", httpapi.ErrValidation)
	}
	if !validType(entry.Type) {
		return fmt.Errorf("%w: unknown credit type %q", httpapi.ErrValidation, entry.Type)
	}
	return nil
}

func validType(t string) bool {
	switch t {
	case TypeEarn, TypeSpend, TypeRefund:
		return true
	default:
		return false
	}
}

func signedMontant(entry Entry) int {
	if entry.Type == TypeSpend {
		return -entry.Amount
	}
	return entry.Amount
}
