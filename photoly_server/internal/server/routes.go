package server

import (
	"github.com/WiscWebGroup/photoly/photoly_server/internal/admin"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/ai"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/catalog"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/docs"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/photo"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/uploads"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/user"
)

// Route packages; a path none of them registers is a 404.
func init() {
	Registrars = append(Registrars, user.Register, catalog.Register, photo.Register, uploads.Register, ai.Register, admin.Register, docs.Register)
	Background = append(Background, uploads.Worker, photo.ScanWorker)
}
