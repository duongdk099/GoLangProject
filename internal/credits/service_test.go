package credits

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"barterswap/pkg/httpapi"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		entry   Entry
		wantErr bool
	}{
		{name: "valid earn", entry: Entry{UserID: 1, Amount: 10, Type: TypeEarn}},
		{name: "valid spend", entry: Entry{UserID: 1, ExchangeID: 3, Amount: 2, Type: TypeSpend}},
		{name: "missing user", entry: Entry{Amount: 10, Type: TypeEarn}, wantErr: true},
		{name: "non positive amount", entry: Entry{UserID: 1, Amount: 0, Type: TypeEarn}, wantErr: true},
		{name: "unknown type", entry: Entry{UserID: 1, Amount: 1, Type: "bonus"}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Validate(test.entry)
			if test.wantErr {
				if !errors.Is(err, httpapi.ErrValidation) {
					t.Fatalf("Validate() error = %v, want validation", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestSignedMontant(t *testing.T) {
	if got := signedMontant(Entry{Amount: 5, Type: TypeSpend}); got != -5 {
		t.Fatalf("signedMontant(spend) = %d, want -5", got)
	}
	if got := signedMontant(Entry{Amount: 5, Type: TypeEarn}); got != 5 {
		t.Fatalf("signedMontant(earn) = %d, want 5", got)
	}
	if got := signedMontant(Entry{Amount: 5, Type: TypeRefund}); got != 5 {
		t.Fatalf("signedMontant(refund) = %d, want 5", got)
	}
}

type fakeExec struct {
	lastArgs   []any
	failUnique bool
}

func (f *fakeExec) ExecContext(_ context.Context, _ string, args ...any) (sql.Result, error) {
	f.lastArgs = args
	if f.failUnique {
		return nil, uniqueViolationError{}
	}
	return driverResult{}, nil
}

func (f *fakeExec) QueryRowContext(_ context.Context, _ string, _ ...any) *sql.Row {
	panic("QueryRowContext should not be called by Record")
}

type driverResult struct{}

func (driverResult) LastInsertId() (int64, error) { return 0, nil }
func (driverResult) RowsAffected() (int64, error) { return 1, nil }

type uniqueViolationError struct{}

func (uniqueViolationError) Error() string    { return "duplicate key value" }
func (uniqueViolationError) SQLState() string { return "23505" }

func TestRecord(t *testing.T) {
	exec := &fakeExec{}
	if err := Record(context.Background(), exec, Entry{UserID: 7, ExchangeID: 3, Amount: 2, Type: TypeSpend}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if got := exec.lastArgs[2]; got != -2 {
		t.Fatalf("recorded montant = %v, want -2", got)
	}
	if got := exec.lastArgs[1]; got != 3 {
		t.Fatalf("recorded exchange_id = %v, want 3", got)
	}
}

func TestRecordWelcomeUsesNullExchange(t *testing.T) {
	exec := &fakeExec{}
	if err := Record(context.Background(), exec, Entry{UserID: 7, Amount: 10, Type: TypeEarn}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if exec.lastArgs[1] != nil {
		t.Fatalf("recorded exchange_id = %v, want nil", exec.lastArgs[1])
	}
}

func TestRecordValidationFailsBeforeInsert(t *testing.T) {
	exec := &fakeExec{}
	if err := Record(context.Background(), exec, Entry{UserID: 0, Amount: 1, Type: TypeEarn}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Record(invalid) error = %v, want validation", err)
	}
	if exec.lastArgs != nil {
		t.Fatal("Record(invalid) must not reach the database")
	}
}

func TestRecordDuplicateIsConflict(t *testing.T) {
	exec := &fakeExec{failUnique: true}
	if err := Record(context.Background(), exec, Entry{UserID: 7, ExchangeID: 3, Amount: 2, Type: TypeSpend}); !errors.Is(err, httpapi.ErrConflict) {
		t.Fatalf("Record(duplicate) error = %v, want conflict", err)
	}
}

type failingExec struct{}

func (failingExec) ExecContext(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	return nil, errors.New("connection reset")
}

func (failingExec) QueryRowContext(_ context.Context, _ string, _ ...any) *sql.Row {
	panic("QueryRowContext should not be called by Record")
}

func TestRecordGeneralErrorIsWrapped(t *testing.T) {
	err := Record(context.Background(), failingExec{}, Entry{UserID: 7, ExchangeID: 3, Amount: 2, Type: TypeSpend})
	if err == nil {
		t.Fatal("Record() expected an error")
	}

	if errors.Is(err, httpapi.ErrValidation) || errors.Is(err, httpapi.ErrConflict) {
		t.Fatalf("Record() error = %v, want a plain wrapped error", err)
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Fatal("isUniqueViolation(nil) = true, want false")
	}
	if isUniqueViolation(errors.New("plain")) {
		t.Fatal("isUniqueViolation(plain) = true, want false")
	}
	if !isUniqueViolation(uniqueViolationError{}) {
		t.Fatal("isUniqueViolation(23505) = false, want true")
	}
}
