package main

import (
	"embed"
	"io/fs"
)

//go:embed web/templates/*.html web/static/*
var webFiles embed.FS

var webStaticFiles = mustWebStaticFiles()

func mustWebStaticFiles() fs.FS {
	files, err := fs.Sub(webFiles, "web/static")
	if err != nil {
		panic(err)
	}
	return files
}
