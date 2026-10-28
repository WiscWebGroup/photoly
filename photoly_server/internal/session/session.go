// Package session stores login tokens in a Postgres UNLOGGED table (replaces Redis).
package session

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store implements user/legacy.SessionStore (same table: photoly_session, see migrations).
type Store struct{ Pool *pgxpool.Pool }

func (s Store) q() *dbgen.Queries { return dbgen.New(s.Pool) }

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
		return token, s.q().CreateSession(ctx, dbgen.CreateSessionParams{Token: token, UserID: uid, Days: int64(days)})
	}
	return token, nil
}

// Lookup resolves a token to its (existing) user, as UserService.identify does.
func (s Store) Lookup(ctx context.Context, token string) (app.User, bool) {
	var u app.User
	if token == "" {
		return u, false
	}
	row, err := s.q().LookupSession(ctx, token)
	if err != nil {
		return u, false
	}
	return app.User{ID: row.UserID, UUID: deref(row.Uuid), Email: deref(row.Email), Role: deref(row.Role)}, true
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
		_ = s.q().DeleteSession(ctx, token)
	}
}

func (s Store) RevokeUser(ctx context.Context, uid int64) {
	_ = s.q().DeleteUserSessions(ctx, uid)
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
			_ = s.q().DeleteExpiredSessions(ctx)
		}
	}
}
