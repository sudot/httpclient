package parse

import (
	"fmt"
	"os"
	"path/filepath"
)

func RecursiveFile(path string) []string {
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		fmt.Printf("文件不存在. %v", path)
		return nil
	}
	var paths []string
	_ = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	return paths
}
