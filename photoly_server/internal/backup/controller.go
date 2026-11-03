package backup

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/session"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Runtime struct {
	Handler http.Handler
	Stop    func()
}
type StartRuntime func(context.Context) (Runtime, error)
type Job struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	BackupID string     `json:"backupId"`
	State    string     `json:"state"`
	Stage    string     `json:"stage"`
	Files    int64      `json:"files"`
	Bytes    int64      `json:"bytes"`
	Started  time.Time  `json:"startedAt"`
	Finished *time.Time `json:"finishedAt,omitempty"`
	Error    string     `json:"error,omitempty"`
}
type State struct {
	PreserveAccounts bool   `json:"preserveAccounts,omitempty"`
	Mode             string `json:"mode"`
	Error            string `json:"error,omitempty"`
	Job              *Job   `json:"job,omitempty"`
}
type Controller struct {
	cfg           app.Config
	engine        *Engine
	ctx           context.Context
	start         StartRuntime
	lock          *os.File
	mu            sync.Mutex
	state         State
	runtime       Runtime
	active        int
	changed       chan struct{}
	transitioning bool
	resume        bool
	recoveryError string
	lastSave      time.Time
	dbMu          sync.RWMutex
	pool          *pgxpool.Pool
	sessions      map[[32]byte]time.Time
	workers       sync.WaitGroup
}

func Open(ctx context.Context, cfg app.Config, start StartRuntime) (*Controller, error) {
	lock, err := prepare(cfg)
	if err != nil {
		return nil, err
	}
	c := &Controller{cfg: cfg, engine: &Engine{Cfg: cfg}, ctx: ctx, start: start, lock: lock, changed: make(chan struct{}), sessions: map[[32]byte]time.Time{}}
	fail := func(err error) (*Controller, error) { lock.Close(); return nil, err }
	c.state.Mode = "off"
	if err = readJSON(c.statePath(), &c.state); err != nil && !os.IsNotExist(err) {
		return fail(err)
	}
	if c.state.Job != nil && c.state.Job.State == "running" {
		c.state.Job.State = "interrupted"
		c.state.Job.Error = "Process stopped; start a new task"
		c.state.Mode = "on"
	}
	if err = c.engine.Recover(ctx); err != nil {
		c.recoveryError = err.Error()
		c.state.Error = "Recovery required: " + err.Error()
		c.state.Mode = "on"
	}
	if c.state.Mode == "draining" {
		c.state.Mode = "on"
	}
	var journal Journal
	if readJSON(c.engine.journalPath(), &journal) == nil && journal.State == "committed" {
		c.state.PreserveAccounts = true
	}
	if c.state.Mode == "off" {
		if c.runtime, err = start(context.WithValue(ctx, preserveAccountsKey{}, c.state.PreserveAccounts)); err != nil {
			return fail(err)
		}
	}
	if err = c.openPool(); err != nil {
		if c.runtime.Stop != nil {
			c.runtime.Stop()
		}
		return fail(err)
	}
	if err = c.saveLocked(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}
func (c *Controller) statePath() string {
	return filepath.Join(c.cfg.BackupDir, ".control", "state.json")
}
func (c *Controller) saveLocked() error {
	if c.state.Job != nil {
		if err := atomicJSON(filepath.Join(c.cfg.BackupDir, ".control", "job-"+c.state.Job.ID+".json"), c.state.Job); err != nil {
			return err
		}
	}
	return atomicJSON(c.statePath(), c.state)
}
func (c *Controller) openPool() error {
	c.dbMu.Lock()
	defer c.dbMu.Unlock()
	pool, err := pgxpool.New(c.ctx, c.cfg.DatabaseURL)
	if err != nil {
		return err
	}
	c.pool = pool
	return nil
}
func (c *Controller) closePool() {
	c.dbMu.Lock()
	defer c.dbMu.Unlock()
	if c.pool != nil {
		c.pool.Close()
		c.pool = nil
	}
}
func (c *Controller) Close() {
	c.workers.Wait()
	c.mu.Lock()
	rt := c.runtime
	c.runtime = Runtime{}
	c.mu.Unlock()
	if rt.Stop != nil {
		rt.Stop()
	}
	c.closePool()
	if c.lock != nil {
		c.lock.Close()
	}
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	code := status
	if status == http.StatusAccepted {
		code = 200
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"msgCode": code, "t": v})
}
func (c *Controller) busyLocked() bool { return c.state.Job != nil && c.state.Job.State == "running" }
func (c *Controller) notifyLocked()    { close(c.changed); c.changed = make(chan struct{}) }
func (c *Controller) authorize(r *http.Request) bool {
	token := r.Header.Get("HRD-Token")
	if token == "" {
		return false
	}
	key := sha256.Sum256([]byte(token))
	c.mu.Lock()
	switching := c.busyLocked() && c.state.Job.Kind == "restore"
	expires := c.sessions[key]
	if switching {
		allowed := r.Method == "GET" && time.Now().Before(expires)
		if allowed {
			c.sessions[key] = time.Now().Add(30 * time.Minute)
		}
		c.mu.Unlock()
		return allowed
	}
	c.mu.Unlock()
	c.dbMu.RLock()
	defer c.dbMu.RUnlock()
	if c.pool == nil {
		return false
	}
	u, ok := (session.Store{Pool: c.pool}).Lookup(r.Context(), token)
	if !ok || u.Role != "admin" || u.Email == c.cfg.DemoEmail {
		return false
	}
	c.mu.Lock()
	c.sessions[key] = time.Now().Add(30 * time.Minute)
	c.mu.Unlock()
	return true
}
func (c *Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	r.URL.Path = path
	if path == "/maintenance/status" && r.Method == "GET" {
		c.mu.Lock()
		mode := c.state.Mode
		c.mu.Unlock()
		reply(w, 200, map[string]string{"mode": mode})
		return
	}
	if path == "/healthz" {
		c.mu.Lock()
		mode := c.state.Mode
		recovery := c.recoveryError
		busy := c.busyLocked()
		c.mu.Unlock()
		if recovery != "" {
			reply(w, 503, map[string]string{"status": "recovery_required"})
			return
		}
		if !busy {
			c.dbMu.RLock()
			err := errors.New("database unavailable")
			if c.pool != nil {
				err = c.pool.Ping(r.Context())
			}
			c.dbMu.RUnlock()
			if err != nil {
				reply(w, 503, "database unavailable")
				return
			}
		}
		reply(w, 200, map[string]string{"status": "ok", "maintenance": mode})
		return
	}
	isControl := path == "/admin/maintenance" || strings.HasPrefix(path, "/admin/maintenance/") || path == "/admin/backups" || strings.HasPrefix(path, "/admin/backups/") || strings.HasPrefix(path, "/admin/backup-jobs/")
	if isControl {
		if !c.authorize(r) {
			reply(w, 403, "Administrator access required")
			return
		}
		c.control(w, r)
		return
	}
	c.mu.Lock()
	mode := c.state.Mode
	if mode == "off" && c.runtime.Handler != nil {
		h := c.runtime.Handler
		c.active++
		c.mu.Unlock()
		defer func() { c.mu.Lock(); c.active--; c.notifyLocked(); c.mu.Unlock() }()
		h.ServeHTTP(w, r)
		return
	}
	c.mu.Unlock()
	if path == "/user/signIn" && r.Method == "POST" {
		c.login(w, r)
		return
	}
	if path == "/admin/isAdmin" || path == "/user/pingToken" {
		if c.authorize(r) {
			if path == "/admin/isAdmin" {
				reply(w, 200, true)
			} else {
				reply(w, 200, 1)
			}
		} else {
			reply(w, 403, "Administrator access required")
		}
		return
	}
	if path == "/demo" && r.Method == "GET" {
		reply(w, 200, map[string]any{"enabled": false})
		return
	}
	reply(w, 503, map[string]string{"reason": "maintenance", "mode": mode})
}
func (c *Controller) login(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	busy := c.busyLocked()
	c.mu.Unlock()
	if busy {
		reply(w, 503, map[string]string{"reason": "maintenance", "message": "Sign in after the current task finishes"})
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&body); err != nil {
		reply(w, 400, "Invalid credentials")
		return
	}
	c.dbMu.RLock()
	defer c.dbMu.RUnlock()
	if c.pool == nil {
		reply(w, 503, "Database unavailable")
		return
	}
	var id int64
	var hash string
	err := c.pool.QueryRow(r.Context(), `SELECT user_id,password FROM "user" WHERE lower(email)=lower($1) AND role='admin'`, body.Email).Scan(&id, &hash)
	if err != nil || body.Email == c.cfg.DemoEmail || !legacy.CheckPassword(hash, body.Password) {
		reply(w, 403, "Invalid administrator credentials")
		return
	}
	token, err := (session.Store{Pool: c.pool}).Create(r.Context(), id, 1)
	if err != nil {
		reply(w, 500, "Sign-in failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"msgCode": 200, "t": "Succeed", "token": token})
}
func (c *Controller) enter() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.Mode != "off" || c.transitioning {
		return errors.New("maintenance transition already in progress")
	}
	c.state.Mode = "draining"
	c.state.Error = ""
	c.transitioning = true
	if err := c.saveLocked(); err != nil {
		c.state.Mode = "off"
		c.transitioning = false
		return err
	}
	c.workers.Add(1)
	go func() {
		defer c.workers.Done()
		done := make(chan struct{})
		go func() {
			timer := time.NewTimer(120 * time.Second)
			defer timer.Stop()
			select {
			case <-done:
				return
			case <-timer.C:
				c.mu.Lock()
				if c.state.Mode == "draining" {
					c.state.Error = "Draining exceeded 120 seconds; waiting for in-flight operations to stop"
					_ = c.saveLocked()
				}
				c.mu.Unlock()
			}
		}()
		c.mu.Lock()
		for c.active > 0 {
			changed := c.changed
			c.mu.Unlock()
			<-changed
			c.mu.Lock()
		}
		rt := c.runtime
		c.runtime = Runtime{}
		c.mu.Unlock()
		if rt.Stop != nil {
			rt.Stop()
		}
		close(done)
		c.mu.Lock()
		c.state.Mode = "on"
		c.state.Error = ""
		c.transitioning = false
		resume := c.resume
		c.resume = false
		if err := c.saveLocked(); err != nil {
			c.state.Error = err.Error()
		}
		c.mu.Unlock()
		if resume {
			_ = c.exit()
		}
	}()
	return nil
}
func (c *Controller) exit() error {
	c.mu.Lock()
	if c.busyLocked() || c.recoveryError != "" {
		c.mu.Unlock()
		return errors.New("a task or recovery is still active")
	}
	if c.state.Mode == "draining" {
		c.resume = true
		c.state.Error = "Returning to service after in-flight operations stop"
		err := c.saveLocked()
		c.mu.Unlock()
		return err
	}
	if c.state.Mode != "on" || c.transitioning {
		c.mu.Unlock()
		return errors.New("maintenance is not ready")
	}
	c.transitioning = true
	c.mu.Unlock()
	rt, err := c.start(context.WithValue(c.ctx, preserveAccountsKey{}, c.state.PreserveAccounts))
	c.mu.Lock()
	defer c.mu.Unlock()
	c.transitioning = false
	if err != nil {
		c.state.Error = err.Error()
		_ = c.saveLocked()
		return err
	}
	c.runtime = rt
	c.state.Mode = "off"
	c.state.Error = ""
	if err = c.saveLocked(); err != nil {
		c.state.Mode = "on"
		c.runtime = Runtime{}
		rt.Stop()
		return err
	}
	return nil
}
func (c *Controller) control(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if r.Method == "GET" && path == "/admin/maintenance" {
		c.mu.Lock()
		copy := c.state
		if copy.Job != nil {
			j := *copy.Job
			copy.Job = &j
		}
		busy := c.busyLocked()
		c.mu.Unlock()
		problems := []string{}
		if !busy {
			problems = c.engine.Capabilities(r.Context())
		}
		rollbacks := []Journal{}
		entries, _ := filepath.Glob(filepath.Join(c.cfg.BackupDir, ".control", "rollback-*.json"))
		for _, p := range entries {
			var j Journal
			if readJSON(p, &j) == nil && j.State == "committed" {
				rollbacks = append(rollbacks, j)
			}
		}
		reply(w, 200, map[string]any{"state": copy, "problems": problems, "backupDirectory": c.cfg.BackupDir, "rollbacks": rollbacks, "externalSources": c.externalSources(r.Context(), busy)})
		return
	}
	if r.Method == "POST" && (path == "/admin/maintenance/enter" || path == "/admin/maintenance/exit") {
		var err error
		if strings.HasSuffix(path, "/enter") {
			err = c.enter()
		} else {
			err = c.exit()
		}
		if err != nil {
			reply(w, 409, err.Error())
			return
		}
		reply(w, 202, map[string]bool{"accepted": true})
		return
	}
	if r.Method == "GET" && path == "/admin/backups" {
		list, err := c.engine.List()
		if err != nil {
			reply(w, 500, "Could not list backups")
			return
		}
		reply(w, 200, list)
		return
	}
	if r.Method == "GET" && strings.HasPrefix(path, "/admin/backup-jobs/") {
		id, err := validatedID(strings.TrimPrefix(path, "/admin/backup-jobs/"))
		if err != nil {
			reply(w, 400, "Invalid job ID")
			return
		}
		var j Job
		if readJSON(filepath.Join(c.cfg.BackupDir, ".control", "job-"+id+".json"), &j) != nil {
			reply(w, 404, "Job not found")
			return
		}
		reply(w, 200, j)
		return
	}
	if r.Method == "POST" && (path == "/admin/backups" || (strings.HasPrefix(path, "/admin/backups/") && strings.HasSuffix(path, "/restore"))) {
		var body struct {
			Password string `json:"password"`
			Confirm  bool   `json:"confirm"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&body); err != nil {
			reply(w, 400, "Invalid request")
			return
		}
		kind, id := "backup", uuid.NewString()
		if path != "/admin/backups" {
			kind = "restore"
			var err error
			id, err = validatedID(strings.TrimSuffix(strings.TrimPrefix(path, "/admin/backups/"), "/restore"))
			if !body.Confirm || err != nil {
				reply(w, 400, "Confirm whole-instance replacement")
				return
			}
		}
		job, err := c.launch(kind, id, body.Password)
		if err != nil {
			reply(w, 409, err.Error())
			return
		}
		reply(w, 202, job)
		return
	}
	reply(w, 404, "Not found")
}
func (c *Controller) launch(kind, id, password string) (Job, error) {
	id, err := validatedID(id)
	if err != nil {
		return Job{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.Mode != "on" || c.transitioning || c.busyLocked() || c.recoveryError != "" {
		return Job{}, errors.New("maintenance must be ready and no task can be running")
	}
	if password == "" || (kind == "backup" && len(password) < 12) {
		return Job{}, errors.New("backup passwords must contain at least 12 characters")
	}
	job := Job{ID: uuid.NewString(), Kind: kind, BackupID: id, State: "running", Stage: "Starting", Started: time.Now().UTC()}
	c.state.Job = &job
	if err := c.saveLocked(); err != nil {
		c.state.Job = nil
		return Job{}, err
	}
	result := job
	c.workers.Add(1)
	go func() {
		defer c.workers.Done()
		engine := *c.engine
		engine.Progress = func(stage string, files, bytes int64) {
			c.mu.Lock()
			defer c.mu.Unlock()
			job.Stage = stage
			job.Files += files
			job.Bytes += bytes
			if time.Since(c.lastSave) > time.Second {
				if err := c.saveLocked(); err != nil {
					log.Printf("backup progress persistence failed: %v", err)
				}
				c.lastSave = time.Now()
			}
		}
		var err error
		if kind == "backup" {
			_, err = engine.Backup(c.ctx, id, password)
		} else {
			c.closePool()
			err = engine.Restore(c.ctx, id, password)
			if re := c.openPool(); err == nil {
				err = re
			}
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		now := time.Now().UTC()
		job.Finished = &now
		job.State = "succeeded"
		job.Stage = "Complete"
		if err != nil {
			job.State = "failed"
			job.Error = err.Error()
			job.Stage = "Failed"
		}
		if kind == "restore" {
			if err == nil {
				c.state.PreserveAccounts = true
			}
			c.sessions = map[[32]byte]time.Time{}
			var j Journal
			if readJSON(engine.journalPath(), &j) == nil && j.State != "committed" && j.State != "rolled_back" {
				c.recoveryError = "Restore rollback requires recovery"
				c.state.Error = c.recoveryError
			}
		}
		if err = c.saveLocked(); err != nil {
			job.State = "failed"
			job.Error = "Could not persist task completion: " + err.Error()
			c.state.Error = job.Error
			log.Print(job.Error)
		}
	}()
	return result, nil
}
func Serve(ctx context.Context, cfg app.Config, start StartRuntime) error {
	c, err := Open(ctx, cfg, start)
	if err != nil {
		return err
	}
	defer c.Close()
	srv := &http.Server{Addr: cfg.Addr, Handler: c, ReadHeaderTimeout: 15 * time.Second}
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	log.Printf("photoly: listening on %s", cfg.Addr)
	select {
	case err = <-done:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}
func Offline(ctx context.Context, cfg app.Config, args []string) error {
	lock, err := prepare(cfg)
	if err != nil {
		return err
	}
	defer lock.Close()
	engine := &Engine{Cfg: cfg, Progress: func(s string, _, _ int64) { log.Print(s) }}
	// CLI reads a password from stdin, so it never appears in argv/history or process listings.
	if len(args) == 1 && args[0] == "rollback" {
		if err = atomicJSON(filepath.Join(cfg.BackupDir, ".control", "state.json"), State{Mode: "on", PreserveAccounts: true}); err != nil {
			return err
		}
		return engine.Rollback(ctx)
	}
	if len(args) != 1 {
		return errors.New("usage: photoly restore BACKUP_ID < password-file | photoly restore rollback")
	}
	if err = atomicJSON(filepath.Join(cfg.BackupDir, ".control", "state.json"), State{Mode: "on", PreserveAccounts: true}); err != nil {
		return err
	}
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return errors.New("read a backup password from stdin")
	}
	password := scanner.Text()
	if err = engine.Recover(ctx); err != nil {
		return err
	}
	return engine.Restore(ctx, args[0], password)
}

type preserveAccountsKey struct{}

// PreserveAccounts prevents deployment bootstrap credentials adding users after a restore.
func PreserveAccounts(ctx context.Context) bool {
	v, _ := ctx.Value(preserveAccountsKey{}).(bool)
	return v
}

func (c *Controller) externalSources(ctx context.Context, busy bool) []string {
	out := []string{}
	if busy {
		return out
	}
	c.dbMu.RLock()
	defer c.dbMu.RUnlock()
	if c.pool == nil {
		return out
	}
	rows, err := c.pool.Query(ctx, `SELECT kind || ': ' || display_name FROM storage_source WHERE NOT(is_default AND kind='local') ORDER BY id`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			out = append(out, name)
		}
	}
	return out
}
