package pgxutil

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterUUIDArrayCodecs maps uuid slice types onto PostgreSQL _uuid (uuid[])
// so pgx can encode under simple/exec protocol (OID 0). Production transaction
// poolers typically cannot use prepared-statement OIDs. RegisterDefaultPgType
// looks up catalog names; "uuid[]" is a silent no-op when only "_uuid" exists.
func RegisterUUIDArrayCodecs(conn *pgx.Conn) {
	if conn == nil {
		return
	}
	m := conn.TypeMap()
	uuidType, ok := m.TypeForOID(pgtype.UUIDOID)
	if !ok {
		uuidType = &pgtype.Type{Name: "uuid", OID: pgtype.UUIDOID, Codec: pgtype.UUIDCodec{}}
		m.RegisterType(uuidType)
	}
	m.RegisterType(&pgtype.Type{
		Name:  "_uuid",
		OID:   pgtype.UUIDArrayOID,
		Codec: &pgtype.ArrayCodec{ElementType: uuidType},
	})
	for _, name := range []string{"_uuid", "uuid[]"} {
		m.RegisterDefaultPgType([]uuid.UUID{}, name)
		m.RegisterDefaultPgType(pgtype.FlatArray[uuid.UUID]{}, name)
	}
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

// formatUUIDArrayLiteral renders Postgres uuid[] text input ({uuid,uuid} / {}).
// SQL already casts $1::uuid[] so binding text does not need a TypeMap OID.
func formatUUIDArrayLiteral(ids []uuid.UUID) string {
	if len(ids) == 0 {
		return "{}"
	}
	var b strings.Builder
	b.Grow(2 + len(ids)*37)
	b.WriteByte('{')
	for i, id := range ids {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(id.String())
	}
	b.WriteByte('}')
	return b.String()
}

// RewriteUUIDSliceArgs converts []uuid.UUID and FlatArray[uuid.UUID] bind
// values to a Postgres uuid[] text literal so transaction poolers (OID 0)
// can encode without a FlatArray codec plan.
func RewriteUUIDSliceArgs(args []any) []any {
	if len(args) == 0 {
		return args
	}
	out := make([]any, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case []uuid.UUID:
			out[i] = formatUUIDArrayLiteral(v)
		case pgtype.FlatArray[uuid.UUID]:
			out[i] = formatUUIDArrayLiteral([]uuid.UUID(v))
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
