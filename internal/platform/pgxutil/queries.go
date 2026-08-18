package pgxutil

import "github.com/avf/avf-vending-api/internal/gen/db"

// NewQueries is db.New with uuid[] bind rewriting. Prefer this over db.New for
// production sqlc constructors that may bind []uuid.UUID (sqlc v1.31.1 still
// emits that type despite sqlc.yaml overrides).
func NewQueries(inner queryDBTX) *db.Queries {
	return db.New(WrapQueryDBTX(inner))
}
