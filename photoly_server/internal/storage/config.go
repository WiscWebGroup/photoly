package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
)

// Config is the persisted backend choice: Driver plus its string settings (secrets included
// in memory, sealed with STORAGE_SECRET_KEY at rest).
type Config struct {
	Driver string            `json:"driver"`
	Values map[string]string `json:"values"`
}

var ErrNoSecretKey = errors.New("STORAGE_SECRET_KEY must be set to save storage credentials")

func gcm() (cipher.AEAD, error) {
	k := os.Getenv("STORAGE_SECRET_KEY")
	if k == "" {
		return nil, ErrNoSecretKey
	}
	sum := sha256.Sum256([]byte(k))
	b, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}

func seal(plain []byte) ([]byte, error) {
	g, err := gcm()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, plain, nil), nil
}

func open(sealed []byte) ([]byte, error) {
	g, err := gcm()
	if err != nil {
		return nil, err
	}
	if len(sealed) < g.NonceSize() {
		return nil, errors.New("storage: corrupt secret")
	}
	return g.Open(nil, sealed[:g.NonceSize()], sealed[g.NonceSize():], nil)
}

func secretNames(driver string) map[string]bool {
	out := map[string]bool{}
	if d, ok := drivers[driver]; ok {
		for _, f := range d.Fields {
			if f.Secret {
				out[f.Name] = true
			}
		}
	}
	return out
}

// Load reads and decrypts the stored config. A secret that cannot be decrypted (key
// missing/changed) is an error, so a bad key never silently falls back to local.
func Load(ctx context.Context, db dbgen.DBTX) (Config, error) {
	row, err := dbgen.New(db).GetStorageConfig(ctx)
	if err != nil {
		return Config{}, err
	}
	c := Config{Driver: row.Driver, Values: map[string]string{}}
	if err = json.Unmarshal(row.Config, &c.Values); err != nil {
		return Config{}, err
	}
	if len(row.Secret) > 0 {
		plain, err := open(row.Secret)
		if err != nil {
			return Config{}, err
		}
		var sec map[string]string
		if err = json.Unmarshal(plain, &sec); err != nil {
			return Config{}, err
		}
		for k, v := range sec {
			c.Values[k] = v
		}
	}
	return c, nil
}

// Save splits secrets from plain values, seals the secrets and stores both.
func Save(ctx context.Context, db dbgen.DBTX, c Config) error {
	secrets, plain := map[string]string{}, map[string]string{}
	names := secretNames(c.Driver)
	for k, v := range c.Values {
		if names[k] {
			secrets[k] = v
		} else {
			plain[k] = v
		}
	}
	var sealed []byte
	if len(secrets) > 0 {
		b, _ := json.Marshal(secrets)
		var err error
		if sealed, err = seal(b); err != nil {
			return err
		}
	}
	cfg, _ := json.Marshal(plain)
	q := dbgen.New(db)
	if err := q.SetStorageConfig(ctx, dbgen.SetStorageConfigParams{Driver: c.Driver, BackendID: BackendID(c.Driver, c.Values), Config: cfg, Secret: sealed}); err != nil {
		return err
	}
	if len(c.Values) == 0 { // nothing to remember (local)
		return nil
	}
	return q.SetStorageDriverConfig(ctx, dbgen.SetStorageDriverConfigParams{Driver: c.Driver, Config: cfg, Secret: sealed})
}

// CurrentID is the saved backend's ID (see BackendID).
func CurrentID(ctx context.Context, db dbgen.DBTX) (string, error) {
	row, err := dbgen.New(db).GetStorageConfig(ctx)
	return row.BackendID, err
}

// SecretKeyConfigured reports whether credentials can be sealed/opened.
func SecretKeyConfigured() bool { return os.Getenv("STORAGE_SECRET_KEY") != "" }

// Saved is a driver's last saved settings, without secrets.
type Saved struct {
	Values    map[string]string `json:"values"`
	HasSecret bool              `json:"hasSecret"`
}

// Public is the active config without secrets, plus the last saved settings of every driver
// (so switching away and back keeps them). It works without STORAGE_SECRET_KEY so the admin
// page can always render.
func Public(ctx context.Context, db dbgen.DBTX) (c Config, saved map[string]Saved, err error) {
	q := dbgen.New(db)
	row, err := q.GetStorageConfig(ctx)
	if err != nil {
		return c, nil, err
	}
	c = Config{Driver: row.Driver, Values: map[string]string{}}
	if err = json.Unmarshal(row.Config, &c.Values); err != nil {
		return c, nil, err
	}
	rows, err := q.ListStorageDriverConfigs(ctx)
	if err != nil {
		return c, nil, err
	}
	saved = map[string]Saved{}
	for _, r := range rows {
		s := Saved{Values: map[string]string{}, HasSecret: len(r.Secret) > 0}
		if err = json.Unmarshal(r.Config, &s.Values); err != nil {
			return c, nil, err
		}
		saved[r.Driver] = s
	}
	return c, saved, nil
}

// Merge fills blank secret fields of c from that driver's last saved settings, so the
// admin can re-save, or switch back to a driver, without retyping credentials the UI never shows.
func Merge(ctx context.Context, db dbgen.DBTX, c Config) (Config, error) {
	names := secretNames(c.Driver)
	missing := false
	for n := range names {
		if c.Values[n] == "" {
			missing = true
		}
	}
	if !missing {
		return c, nil
	}
	rows, err := dbgen.New(db).ListStorageDriverConfigs(ctx)
	if err != nil {
		return c, err
	}
	for _, r := range rows {
		if r.Driver != c.Driver || len(r.Secret) == 0 {
			continue
		}
		plain, err := open(r.Secret)
		if err != nil {
			return c, err
		}
		var sec map[string]string
		if err = json.Unmarshal(plain, &sec); err != nil {
			return c, err
		}
		for n := range names {
			if c.Values[n] == "" {
				c.Values[n] = sec[n]
			}
		}
	}
	return c, nil
}

// Verify builds the store for c and checks it can be used, without activating it.
func Verify(ctx context.Context, c Config) (Store, error) {
	st, err := Build(c.Driver, c.Values)
	if err != nil {
		return nil, err
	}
	return st, st.Check(ctx)
}

// Startup loads the saved config and activates it. It does not Check the remote, so a brief
// provider outage cannot keep the server from booting; a bad config or key is an error.
func Startup(ctx context.Context, db dbgen.DBTX) error {
	c, err := Load(ctx, db)
	if err != nil {
		return err
	}
	st, err := Build(c.Driver, c.Values)
	if err != nil {
		return err
	}
	Set(c.Driver, st)
	return nil
}
