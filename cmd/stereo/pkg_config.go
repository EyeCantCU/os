package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"chainguard.dev/melange/pkg/build"
	"chainguard.dev/melange/pkg/config"
)

func compilePkgConfig(ctx context.Context, sourceDir string) (*config.Configuration, error) {
	configPath := fmt.Sprintf("%s.yaml", sourceDir)
	pipelineDir := filepath.Join(filepath.Dir(sourceDir), "pipelines")

	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create package source dir %s: %w", sourceDir, err)
	}

	bc, err := build.New(
		ctx,
		build.WithConfigFileRepositoryURL("unused"),
		build.WithConfigFileRepositoryCommit("unused"),
		build.WithConfig(configPath),
		build.WithSourceDir(sourceDir),
		build.WithPipelineDir(pipelineDir),
	)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := bc.Close(ctx); err != nil {
			log.Printf("failed to close melange build context: %v", err)
		}
	}()

	if err := bc.Compile(ctx); err != nil {
		return nil, err
	}

	return bc.Configuration, nil
}

func parseUsedPipelines(cfg *config.Configuration, subcmd string) ([][]config.Pipeline, error) {
	pipelines := [][]config.Pipeline{}

	switch subcmd {
	case "package", "debug":
		pipelines = append(pipelines, cfg.Pipeline)
		for _, subpkg := range cfg.Subpackages {
			pipelines = append(pipelines, subpkg.Pipeline)
		}
	case "test", "test-debug":
		if cfg.Test != nil {
			pipelines = append(pipelines, cfg.Test.Pipeline)
		}
		for _, subpkg := range cfg.Subpackages {
			if subpkg.Test != nil {
				pipelines = append(pipelines, subpkg.Test.Pipeline)
			}
		}
	default:
		return nil, nil
	}

	return pipelines, nil
}

func findPipeline(pipelines []config.Pipeline, match func(string) bool) bool {
	for _, pipeline := range pipelines {
		if match(pipeline.Uses) {
			return true
		}
		if findPipeline(pipeline.Pipeline, match) {
			return true
		}
	}

	return false
}
