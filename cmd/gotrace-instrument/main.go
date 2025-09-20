// gotrace-instrument is a tool for automatically instrumenting Go code with devtrace functionality
package main

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var (
		srcDir     = flag.String("src", ".", "Source directory to instrument")
		outputDir  = flag.String("out", "", "Output directory (default: overwrite source)")
		pattern    = flag.String("pattern", "*.go", "File pattern to match")
		exclude    = flag.String("exclude", "_test.go,vendor/", "Comma-separated patterns to exclude")
		dryRun     = flag.Bool("dry-run", false, "Show what would be changed without making changes")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging")
		addTrace   = flag.Bool("add-trace", true, "Add function tracing")
		addLogging = flag.Bool("add-logging", true, "Add enhanced logging to existing log calls")
	)
	flag.Parse()

	if *outputDir == "" {
		*outputDir = *srcDir
	}

	excludePatterns := strings.Split(*exclude, ",")

	instrumenter := &Instrumenter{
		OutputDir:       *outputDir,
		ExcludePatterns: excludePatterns,
		DryRun:          *dryRun,
		Verbose:         *verbose,
		AddTrace:        *addTrace,
		AddLogging:      *addLogging,
	}

	err := filepath.Walk(*srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		if match, matchErr := filepath.Match(*pattern, filepath.Base(path)); matchErr != nil {
			return matchErr
		} else if !match {
			return nil
		}

		// Check exclusion patterns
		for _, excludePattern := range excludePatterns {
			if excludePattern != "" && strings.Contains(path, excludePattern) {
				if *verbose {
					log.Printf("Excluding: %s (matches %s)", path, excludePattern)
				}
				return nil
			}
		}

		return instrumenter.InstrumentFile(path)
	})

	if err != nil {
		log.Fatalf("Error instrumenting files: %v", err)
	}

	fmt.Println("Instrumentation complete!")
}

type Instrumenter struct {
	OutputDir       string
	ExcludePatterns []string
	DryRun          bool
	Verbose         bool
	AddTrace        bool
	AddLogging      bool
}

func (i *Instrumenter) InstrumentFile(filePath string) error {
	if i.Verbose {
		log.Printf("Processing: %s", filePath)
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", filePath, err)
	}

	transformer := &ASTTransformer{
		FileSet:    fset,
		AddTrace:   i.AddTrace,
		AddLogging: i.AddLogging,
		Verbose:    i.Verbose,
	}

	modified := transformer.Transform(node)

	if !modified {
		if i.Verbose {
			log.Printf("No changes needed for: %s", filePath)
		}
		return nil
	}

	if i.DryRun {
		log.Printf("Would modify: %s", filePath)
		return nil
	}

	// Write the modified file
	outputPath := i.getOutputPath(filePath)
	return transformer.WriteFile(outputPath, node)
}

func (i *Instrumenter) getOutputPath(inputPath string) string {
	if i.OutputDir == filepath.Dir(inputPath) {
		return inputPath // Overwrite original
	}

	// Create relative path structure in output directory
	rel, err := filepath.Rel(".", inputPath)
	if err != nil {
		rel = inputPath
	}

	return filepath.Join(i.OutputDir, rel)
}
