package framework

import (
	"path/filepath"
	"runtime"
)

var (
	_, sourceFile, _, _ = runtime.Caller(0)

	// Root folder of this project
	RepoRoot = filepath.Join(filepath.Dir(sourceFile), "../../..")
)
