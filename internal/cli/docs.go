package cli

import (
	"github.com/carlos/rw-mcp/internal/clidocs"
	"github.com/spf13/cobra"
)

func newDocsCommand() *cobra.Command {
	var outputDirectory string
	var format string
	var frontmatter bool

	command := &cobra.Command{
		Use:     "docs",
		Short:   "Generate CLI documentation",
		Long:    "Generate one stable reference file per command in Markdown, man, or reStructuredText format.",
		GroupID: "reference",
		Example: "  rw-mcp docs\n" +
			"  rw-mcp docs --format markdown --frontmatter --out docs/cli\n" +
			"  rw-mcp docs --format man --out ./man",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return clidocs.Generate(NewRootCommand, outputDirectory, format, frontmatter)
		},
	}
	command.Flags().StringVar(&outputDirectory, "out", "./docs/cli", "Output directory")
	command.Flags().StringVar(&format, "format", "markdown", "Documentation format: markdown, man, or rest")
	command.Flags().BoolVar(&frontmatter, "frontmatter", false, "Prepend simple YAML front matter to Markdown files")
	return command
}
