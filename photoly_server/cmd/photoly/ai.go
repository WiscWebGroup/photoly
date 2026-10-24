package main

import (
	"context"
	"os"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/ai"
)

func init() {
	aiWorker = func(context.Context) error { return ai.Main([]string{"ai-worker"}) }
	aiVerify = func(context.Context) error { return ai.Main(append([]string{"verify"}, os.Args[2:]...)) }
}
