// Package webui embeds the built frontend (frontend/ -> vite build -> ./dist).
package webui

import "embed"

// Dist holds the Vite build output. In dev mode it only contains .keep.
//
//go:embed all:dist
var Dist embed.FS
