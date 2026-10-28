package ai

// Model registry. Qwen is gone: SigLIP2 on ONNX Runtime is the only
// provider, and startup refuses while a Qwen profile is still active.

import (
	"context"
	"fmt"
	"maps"
	"time"
)

func profileSpec(provider any) (map[string]any, error) {
	if p, _ := provider.(string); p == "siglip2" {
		return maps.Clone(ONNXProfile().Spec), nil
	}
	return nil, fail("unsupported_provider")
}

// onnxCompatible: the profile's vectors can be queried with this binary's ONNX encoder.
// That is the ONNX profile itself, or an earlier PyTorch SigLIP2 profile of the same base
// model (cosine of ONNX vs PyTorch is >= 0.9998 in the POC), so search keeps working on the
// old index while the ONNX profile builds.
func onnxCompatible(spec map[string]any) bool {
	if spec["provider"] != "siglip2" || spec["model_id"] != baseModelID || spec["revision"] != baseModelRevision {
		return false
	}
	return true
}

type profileRow struct {
	ID          int64
	Provider    string
	Dimension   int
	Fingerprint string
	Spec        map[string]any
	IndexStatus string
	CreatedAt   time.Time
}

const profileCols = `profile_id, provider, dimension, fingerprint, spec, index_status, created_at`

func (p *profileRow) scan(r interface{ Scan(...any) error }) error {
	return r.Scan(&p.ID, &p.Provider, &p.Dimension, &p.Fingerprint, &p.Spec, &p.IndexStatus, &p.CreatedAt)
}

func getProfile(ctx context.Context, q Q, id int64) (*profileRow, error) {
	p := &profileRow{}
	if err := p.scan(q.QueryRow(ctx, `SELECT `+profileCols+` FROM embedding_profile WHERE profile_id=$1`, id)); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *profileRow) dto() map[string]any {
	return map[string]any{"profileId": p.ID, "provider": p.Provider, "dimension": p.Dimension, "fingerprint": p.Fingerprint,
		"spec": p.Spec, "indexStatus": p.IndexStatus, "createdAt": iso(p.CreatedAt)}
}

// CheckStartup refuses to start while a Qwen profile is in use (settings or any profile row
// of the old provider that is still referenced).
func CheckStartup(ctx context.Context, q Q) error {
	s, err := loadSettings(ctx, q)
	if err != nil {
		return err
	}
	for _, k := range []string{"search_profile_id", "building_profile_id", "loaded_profile_id"} {
		id := s.PID(k)
		if id == nil {
			continue
		}
		p, err := getProfile(ctx, q, *id)
		if err != nil {
			continue
		}
		if p.Provider == "qwen3_vl" {
			return fmt.Errorf("the active AI profile %d uses Qwen3-VL, which this version no longer supports: "+
				"activate a SigLIP2 profile with the previous release (or reset the AI settings) before upgrading", p.ID)
		}
	}
	return nil
}
