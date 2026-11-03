package webembed

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var content embed.FS

func Files() fs.FS { f, _ := fs.Sub(content, "dist"); return f }
