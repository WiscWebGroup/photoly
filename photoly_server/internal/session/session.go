// Package session stores login tokens in a Postgres UNLOGGED table (replaces Redis).
package session

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store implements user/legacy.SessionStore (same table: photoly_session, see migrations).
type Store struct{ Pool *pgxpool.Pool }

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (s Store) Create(ctx context.Context, uid int64, days int) (string, error) {
	token := newToken()
	// Zero days stays accepted and yields an immediately expired session, like Flask.
	if days > 0 {
		_, err := s.Pool.Exec(ctx, `INSERT INTO photoly_session VALUES ($1,$2, now() + make_interval(days => $3))`, token, uid, days)
		return token, err
	}
	return token, nil
}

// Lookup resolves a token to its (existing) user, as UserService.identify does.
func (s Store) Lookup(ctx context.Context, token string) (app.User, bool) {
	var u app.User
	if token == "" {
		return u, false
	}
	var email, role *string
	var uuid *string
	err := s.Pool.QueryRow(ctx, `SELECT u.user_id, u.uuid, u.email, u.role FROM photoly_session s
		JOIN "user" u ON u.user_id=s.user_id WHERE s.token=$1 AND s.expires_at>now()`, token).Scan(&u.ID, &uuid, &email, &role)
	if err != nil {
		return u, false
	}
	u.UUID, u.Email, u.Role = deref(uuid), deref(email), deref(role)
	return u, true
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s Store) Identify(ctx context.Context, token string) (int64, bool) {
	u, ok := s.Lookup(ctx, token)
	return u.ID, ok
}

func (s Store) Delete(ctx context.Context, token string) {
	if token != "" {
		_, _ = s.Pool.Exec(ctx, `DELETE FROM photoly_session WHERE token=$1`, token)
	}
}

func (s Store) RevokeUser(ctx context.Context, uid int64) {
	_, _ = s.Pool.Exec(ctx, `DELETE FROM photoly_session WHERE user_id=$1`, uid)
}

// Sweep deletes expired rows hourly until ctx ends.
func (s Store) Sweep(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, _ = s.Pool.Exec(ctx, `DELETE FROM photoly_session WHERE expires_at<=now()`)
		}
	}
}
