package database

import (
	"strings"
	"testing"
)

func TestEmbeddedSchemaContainsExpectedTables(t *testing.T) {
	for _, table := range []string{"users", "skills", "credit_transactions", "services", "reviews"} {
		if !strings.Contains(Schema, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("embedded schema does not define %s", table)
		}
	}
}
