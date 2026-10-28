package ai

// Main runs the AI subcommands that are not the HTTP server:
//
//	ai-worker  server-side inference over pipes (fds 3 and 4); started by `photoly serve`
//	verify     download the pinned model files (when missing) and check their SHA-256;
//	           prints the profile fingerprint. PHOTOLY_DATA/models is the cache.
import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func Main(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: photoly [ai-worker|verify]")
	}
	switch args[0] {
	case "ai-worker":
		return runWorker()
	case "verify":
		return verify(context.Background())
	}
	return fmt.Errorf("unknown ai command %q", args[0])
}

func verify(ctx context.Context) error {
	profile := ONNXProfile()
	cache := newModelCache(filepath.Join(dataDir(), "models"))
	for name := range modelFiles {
		if _, err := cache.Ensure(ctx, name); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "ok  %s\n", name)
	}
	fmt.Fprintf(os.Stdout, "fingerprint %s\n", profile.Fingerprint)
	return nil
}
