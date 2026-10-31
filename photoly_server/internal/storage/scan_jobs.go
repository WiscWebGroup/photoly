package storage

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ScanJob struct {
	ID          int64
	NamespaceID *int64
	Force       bool
}

// All queue edits use the source row for a short transaction. Network work never
// runs here. Disjoint subtree requests remain separate jobs.
func QueueScan(ctx context.Context, pool *pgxpool.Pool, uid, sid int64, ns *int64, force bool) (ScanJob, error) {
	tx, e := pool.Begin(ctx)
	if e != nil {
		return ScanJob{}, e
	}
	defer tx.Rollback(ctx)
	var owned bool
	e = tx.QueryRow(ctx, `SELECT enabled AND kind<>'s3' AND user_id=$2 FROM storage_source WHERE id=$1 FOR UPDATE`, sid, uid).Scan(&owned)
	if e != nil || !owned {
		return ScanJob{}, errors.New("storage_source_not_owned")
	}
	if ns != nil {
		var source int64
		var mount bool
		e = tx.QueryRow(ctx, `SELECT source_id,is_mount OR ns_parent_id=-1 FROM namespace WHERE ns_id=$1 AND user_id=$2`, *ns, uid).Scan(&source, &mount)
		if e != nil || source != sid {
			return ScanJob{}, errors.New("scan_folder_not_owned")
		}
		if mount {
			ns = nil
		}
	}
	ancestors := func(id *int64) (map[int64]bool, error) {
		out := map[int64]bool{}
		if id == nil {
			return out, nil
		}
		rows, e := tx.Query(ctx, `WITH RECURSIVE a AS(SELECT ns_id,ns_parent_id FROM namespace WHERE ns_id=$1 UNION ALL SELECT n.ns_id,n.ns_parent_id FROM namespace n JOIN a ON a.ns_parent_id=n.ns_id WHERE n.source_id=$2) SELECT ns_id FROM a`, *id, sid)
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		for rows.Next() {
			var i int64
			if e = rows.Scan(&i); e != nil {
				return nil, e
			}
			out[i] = true
		}
		return out, rows.Err()
	}
	rows, e := tx.Query(ctx, `SELECT id,namespace_id,force FROM storage_scan_job WHERE source_id=$1 AND state='queued' ORDER BY id`, sid)
	if e != nil {
		return ScanJob{}, e
	}
	jobs := []ScanJob{}
	for rows.Next() {
		var j ScanJob
		if e = rows.Scan(&j.ID, &j.NamespaceID, &j.Force); e != nil {
			rows.Close()
			return ScanJob{}, e
		}
		jobs = append(jobs, j)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return ScanJob{}, e
	}
	a, e := ancestors(ns)
	if e != nil {
		return ScanJob{}, e
	}
	for _, j := range jobs {
		if j.NamespaceID == nil || ns != nil && a[*j.NamespaceID] {
			_, e = tx.Exec(ctx, `UPDATE storage_scan_job SET force=force OR $2 WHERE id=$1`, j.ID, force)
			if e != nil {
				return j, e
			}
			j.Force = j.Force || force
			return j, tx.Commit(ctx)
		}
	}
	job := ScanJob{NamespaceID: ns, Force: force}
	for _, j := range jobs {
		b, e := ancestors(j.NamespaceID)
		if e != nil {
			return job, e
		}
		if ns == nil || j.NamespaceID != nil && b[*ns] {
			job.Force = job.Force || j.Force
			_, e = tx.Exec(ctx, `DELETE FROM storage_scan_job WHERE id=$1`, j.ID)
			if e != nil {
				return job, e
			}
		}
	}
	e = tx.QueryRow(ctx, `INSERT INTO storage_scan_job(source_id,namespace_id,force) VALUES($1,$2,$3) RETURNING id`, sid, job.NamespaceID, job.Force).Scan(&job.ID)
	if e != nil {
		return job, e
	}
	return job, tx.Commit(ctx)
}
func ClaimScan(ctx context.Context, pool *pgxpool.Pool, sid int64) (ScanJob, error) {
	tx, e := pool.Begin(ctx)
	if e != nil {
		return ScanJob{}, e
	}
	defer tx.Rollback(ctx)
	var full, force bool
	if e = tx.QueryRow(ctx, `SELECT scan_requested,scan_force FROM storage_source WHERE id=$1 FOR UPDATE`, sid).Scan(&full, &force); e != nil {
		return ScanJob{}, e
	}
	j := ScanJob{}
	e = tx.QueryRow(ctx, `SELECT id,namespace_id,force FROM storage_scan_job WHERE source_id=$1 AND state='queued' ORDER BY id LIMIT 1`, sid).Scan(&j.ID, &j.NamespaceID, &j.Force)
	if e != nil && e != pgx.ErrNoRows {
		return j, e
	}
	if full {
		j.NamespaceID = nil
		j.Force = j.Force || force
	}
	if j.ID == 0 {
		e = tx.QueryRow(ctx, `INSERT INTO storage_scan_job(source_id,force,state) VALUES($1,$2,'running') RETURNING id`, sid, j.Force).Scan(&j.ID)
	} else {
		_, e = tx.Exec(ctx, `UPDATE storage_scan_job SET state='running',namespace_id=$2,force=$3 WHERE id=$1`, j.ID, j.NamespaceID, j.Force)
	}
	if e != nil {
		return j, e
	}
	_, e = tx.Exec(ctx, `UPDATE storage_source SET scan_requested=false,scan_force=false,scan_status='scanning',scan_phase='enumerating',scan_error=NULL,scan_started_at=now(),scan_completed=0,scan_total=0,scan_discovered=0,scan_waiting=0,scan_unchanged=0 WHERE id=$1`, sid)
	if e != nil {
		return j, e
	}
	return j, tx.Commit(ctx)
}
