package clidocs

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

func TestGenerateConcurrently(t *testing.T) {
	const workers = 8
	var waitGroup sync.WaitGroup
	errors := make(chan error, workers)

	for range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			outputDirectory := filepath.Join(t.TempDir(), "docs")
			err := Generate(func() *cobra.Command {
				root := &cobra.Command{Use: "sample"}
				root.AddCommand(&cobra.Command{Use: "version"})
				return root
			}, outputDirectory, "markdown", false)
			if err != nil {
				errors <- err
			}
		}()
	}

	waitGroup.Wait()
	close(errors)
	for err := range errors {
		t.Errorf("Generate() error = %v", err)
	}
}
