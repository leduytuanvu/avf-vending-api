package pgxutil

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterUUIDArrayCodecs maps []uuid.UUID onto PostgreSQL uuid[] so pgx can
// encode under simple/text protocol (OID 0). Production transaction poolers
// typically cannot use prepared-statement OIDs.
func RegisterUUIDArrayCodecs(conn *pgx.Conn) {
	if conn == nil {
		return
	}
	m := conn.TypeMap()
	m.RegisterDefaultPgType([]uuid.UUID{}, "uuid[]")
	m.RegisterDefaultPgType(pgtype.FlatArray[uuid.UUID]{}, "uuid[]")
}

// ApplyUUIDArrayCodecsToPoolConfig registers codecs on every acquired connection.
func ApplyUUIDArrayCodecsToPoolConfig(pcfg *pgxpool.Config) {
	if pcfg == nil {
		return
	}
	prev := pcfg.AfterConnect
	pcfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		RegisterUUIDArrayCodecs(conn)
		if prev != nil {
			return prev(ctx, conn)
		}
		return nil
	}
}

// RewriteUUIDSliceArgs converts []uuid.UUID bind values to pgtype.FlatArray
// which has a text encode plan independent of prepared-statement OIDs.
func RewriteUUIDSliceArgs(args []any) []any {
	if len(args) == 0 {
		return args
	}
	out := make([]any, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case []uuid.UUID:
			out[i] = pgtype.FlatArray[uuid.UUID](v)
		default:
			out[i] = a
		}
	}
	return out
}

type queryDBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type rewritingQueryDBTX struct {
	inner queryDBTX
}

func (w rewritingQueryDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return w.inner.Exec(ctx, sql, RewriteUUIDSliceArgs(args)...)
}

func (w rewritingQueryDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return w.inner.Query(ctx, sql, RewriteUUIDSliceArgs(args)...)
}

func (w rewritingQueryDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return w.inner.QueryRow(ctx, sql, RewriteUUIDSliceArgs(args)...)
}

// WrapQueryDBTX rewrites uuid[] bind values for sqlc Queries (internal/gen/db.New).
func WrapQueryDBTX(inner queryDBTX) rewritingQueryDBTX {
	if w, ok := inner.(rewritingQueryDBTX); ok {
		return w
	}
	return rewritingQueryDBTX{inner: inner}
}
