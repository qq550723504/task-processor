package tests

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type goFileIndex struct {
	files map[string]goFileFacts
}

type goFileFacts struct {
	source  []byte
	imports map[string]struct{}
}

var (
	goFileIndexCacheMu sync.Mutex
	goFileIndexCache   = make(map[string]*goFileIndex)
)

func loadGoFileIndex(root, skipRoot string) (*goFileIndex, error) {
	root = filepath.Clean(root)
	skipRoot = filepath.Clean(skipRoot)
	cacheKey := root + "::" + skipRoot

	goFileIndexCacheMu.Lock()
	if cached := goFileIndexCache[cacheKey]; cached != nil {
		goFileIndexCacheMu.Unlock()
		return cached, nil
	}
	goFileIndexCacheMu.Unlock()

	index := &goFileIndex{
		files: make(map[string]goFileFacts),
	}

	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		path = filepath.Clean(path)
		if entry.IsDir() {
			if skipRoot != "" && path == skipRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(fset, path, source, parser.ImportsOnly)
		if err != nil {
			return err
		}

		facts := goFileFacts{
			source:  source,
			imports: make(map[string]struct{}, len(file.Imports)),
		}
		for _, imp := range file.Imports {
			facts.imports[imp.Path.Value] = struct{}{}
		}

		index.files[path] = facts
		return nil
	})
	if err != nil {
		return nil, err
	}

	goFileIndexCacheMu.Lock()
	goFileIndexCache[cacheKey] = index
	goFileIndexCacheMu.Unlock()

	return index, nil
}
