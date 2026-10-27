package legacy

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionStore is what the user routes need from the session layer. The skeleton may
// assign its own implementation to Sessions before Register; otherwise Register installs
// the Postgres UNLOGGED table store below.
type SessionStore interface {
	Create(ctx context.Context, userID int64, days int) (string, error)
	Identify(ctx context.Context, token string) (int64, bool)
	Delete(ctx context.Context, token string)
	RevokeUser(ctx context.Context, userID int64)
}

var Sessions SessionStore

func NewUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type pgSessions struct{ pool *pgxpool.Pool }

// UseDefaultSessions installs the table-backed store unless one is already set.
func UseDefaultSessions(ctx context.Context, pool *pgxpool.Pool) error {
	if Sessions != nil {
		return nil
	}
	_, err := pool.Exec(ctx, `CREATE UNLOGGED TABLE IF NOT EXISTS photoly_session
		(token text PRIMARY KEY, user_id integer NOT NULL, expires_at timestamptz NOT NULL)`)
	Sessions = pgSessions{pool}
	return err
}

func (s pgSessions) Create(ctx context.Context, uid int64, days int) (string, error) {
	token := NewUUID()
	// A duration of zero stays accepted, producing an immediately expired session.
	if days > 0 {
		_, err := s.pool.Exec(ctx, `INSERT INTO photoly_session VALUES ($1,$2, now() + make_interval(days => $3))`, token, uid, days)
		return token, err
	}
	return token, nil
}

func (s pgSessions) Identify(ctx context.Context, token string) (int64, bool) {
	if token == "" {
		return 0, false
	}
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT s.user_id FROM photoly_session s JOIN "user" u ON u.user_id=s.user_id
		WHERE s.token=$1 AND s.expires_at>now()`, token).Scan(&id)
	return id, err == nil
}

func (s pgSessions) Delete(ctx context.Context, token string) {
	if token != "" {
		_, _ = s.pool.Exec(ctx, `DELETE FROM photoly_session WHERE token=$1`, token)
	}
}

func (s pgSessions) RevokeUser(ctx context.Context, uid int64) {
	_, _ = s.pool.Exec(ctx, `DELETE FROM photoly_session WHERE user_id=$1`, uid)
}
