package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root := "internal/productjson"

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			oldContent := string(content)
			newContent := strings.ReplaceAll(oldContent, "product-json-generator/internal", "task-processor/internal/productjson")

			if oldContent != newContent {
				fmt.Printf("Updating: %s\n", path)
				err = ioutil.WriteFile(path, []byte(newContent), info.Mode())
				if err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Import paths updated successfully!")
}
