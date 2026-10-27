// Package photolyserver embeds the frozen OpenAPI document.
package photolyserver

import _ "embed"

//go:embed openapi.json
var OpenAPI []byte
