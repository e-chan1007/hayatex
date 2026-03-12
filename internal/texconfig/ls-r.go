package texconfig

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

var trees = []string{"texmf-dist", "texmf-var", "texmf-config"}

func GenerateLsR(texdir string) error {
	for _, tree := range trees {
		rootPath := filepath.Join(texdir, tree)
		if _, err := os.Stat(rootPath); err == nil {
			err := generateLsR(rootPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func generateLsR(root string) error {
	out, err := os.Create(filepath.Join(root, "ls-R"))
	if err != nil {
		return err
	}
	defer out.Close()
	fmt.Fprintln(out, "% ls-R -- filename database for kpathsea; do not change this line.")
	db := make(map[string][]string)

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == "ls-R" {
			return err
		}
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		dirKey := "./" + filepath.ToSlash(rel)
		db[dirKey] = append(db[dirKey], d.Name())
		return nil
	})

	if err != nil {
		return err
	}

	// ソートして出力：まずディレクトリ名をソートし、各ディレクトリ内のファイル名もソートする
	dirs := make([]string, 0, len(db))
	for k := range db {
		dirs = append(dirs, k)
	}
	slices.Sort(dirs)

	for _, dir := range dirs {
		files := db[dir]
		slices.Sort(files)
		fmt.Fprintf(out, "\n%s:\n", dir)
		for _, f := range files {
			fmt.Fprintln(out, f)
		}
	}
	return nil
}
