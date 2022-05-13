package main

import (
	"io/fs"
	"path/filepath"
)

func FindWatchFolders(watch []string) []string {
	uniqueFolders := map[string]struct{}{}

	for _, folder := range watch {
		directories := getDirectories(folder)
		for _, f := range directories {
			uniqueFolders[f] = struct{}{}
		}
	}

	folders := make([]string, 0, len(uniqueFolders))
	for k := range uniqueFolders {
		folders = append(folders, k)
	}

	return folders
}

func getDirectories(root string) []string {
	var directories []string
	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			directories = append(directories, path)
		}

		return nil
	})
	if err != nil {
		return directories
	}

	return directories
}
