package clidocs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func Generate(rootCommandFactory func() *cobra.Command, outputDirectory, format string, frontmatter bool) error {
	if rootCommandFactory == nil {
		return fmt.Errorf("Cobra root command factory must not be nil")
	}
	root := rootCommandFactory()
	if root == nil {
		return fmt.Errorf("Cobra root command factory returned nil")
	}
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		return fmt.Errorf("create documentation output directory: %w", err)
	}

	switch format {
	case "markdown":
		if frontmatter {
			prep := func(filename string) string {
				base := filepath.Base(filename)
				name := strings.TrimSuffix(base, filepath.Ext(base))
				title := strings.ReplaceAll(name, "_", " ")
				return fmt.Sprintf("---\ntitle: %q\nslug: %q\ndescription: \"CLI reference for %s\"\n---\n\n", title, name, title)
			}
			link := func(name string) string { return strings.ToLower(name) }
			if err := doc.GenMarkdownTreeCustom(root, outputDirectory, prep, link); err != nil {
				return fmt.Errorf("generate Markdown documentation: %w", err)
			}
		} else if err := doc.GenMarkdownTree(root, outputDirectory); err != nil {
			return fmt.Errorf("generate Markdown documentation: %w", err)
		}
	case "man":
		header := &doc.GenManHeader{Title: strings.ToUpper(root.Name()), Section: "1"}
		if err := doc.GenManTree(root, header, outputDirectory); err != nil {
			return fmt.Errorf("generate man documentation: %w", err)
		}
	case "rest":
		if err := doc.GenReSTTree(root, outputDirectory); err != nil {
			return fmt.Errorf("generate reStructuredText documentation: %w", err)
		}
	default:
		return fmt.Errorf("unknown documentation format %q; choose markdown, man, or rest", format)
	}
	return nil
}
