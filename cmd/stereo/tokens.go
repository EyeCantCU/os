package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"chainguard.dev/melange/pkg/build"
	"chainguard.dev/melange/pkg/config"
)

func ensureTokens(ctx context.Context, dir, pkg, subcmd string) error {
	sourceDir := filepath.Join(dir, pkg)

	cfg, err := compilePkgConfig(ctx, sourceDir)
	if err != nil {
		return err
	}

	needsRepo, needsLibraries, err := tokensNeeded(cfg, subcmd)
	if err != nil {
		return err
	}

	if needsRepo {
		if err := generateGitHubToken(ctx, sourceDir); err != nil {
			return err
		}
	}

	if needsLibraries {
		if err := generateLibrariesToken(ctx, sourceDir); err != nil {
			return err
		}
	}

	return nil
}

func compilePkgConfig(ctx context.Context, sourceDir string) (*config.Configuration, error) {
	configPath := fmt.Sprintf("%s.yaml", sourceDir)
	pipelineDir := filepath.Join(filepath.Dir(sourceDir), "pipelines")

	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create package source dir %s: %w", sourceDir, err)
	}

	bc, err := build.New(
		ctx,
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

func tokensNeeded(cfg *config.Configuration, subcmd string) (bool, bool, error) {
	pipelines, err := parseUsedPipelines(cfg, subcmd)
	if err != nil {
		return false, false, err
	}

	needsRepoToken := false
	needsLibrariesToken := false

	for _, pipeline := range pipelines {
		if findAuthPipeline(pipeline, isGitHubPipeline) {
			needsRepoToken = true
		}
		if findAuthPipeline(pipeline, isLibrariesPipeline) {
			needsLibrariesToken = true
		}
	}

	return needsRepoToken, needsLibrariesToken, nil
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

func findAuthPipeline(pipelines []config.Pipeline, match func(string) bool) bool {
	for _, pipeline := range pipelines {
		if match(pipeline.Uses) {
			return true
		}
		if findAuthPipeline(pipeline.Pipeline, match) {
			return true
		}
	}

	return false
}

func isGitHubPipeline(uses string) bool {
	return uses == "auth/github"
}

func isLibrariesPipeline(uses string) bool {
	return strings.HasPrefix(uses, "auth/") && !strings.HasSuffix(uses, "github")
}

func generateGitHubToken(ctx context.Context, sourceDir string) error {
	log.Printf("creating github token in %s", sourceDir)
	dest := filepath.Join(sourceDir, ".github.token")
	args := []string{"auth", "octo-sts", "--identity=guarded-package-repos", "--scope=chainguard-dev"}
	return writeToken(ctx, dest, args)
}

func generateLibrariesToken(ctx context.Context, sourceDir string) error {
	log.Printf("creating libraries token in %s", sourceDir)
	dest := filepath.Join(sourceDir, ".libraries.token")
	args := []string{"auth", "token", "--audience", "libraries.cgr.dev"}
	return writeToken(ctx, dest, args)
}

func writeToken(ctx context.Context, dest string, args []string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create token directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".tmp")
	if err != nil {
		return fmt.Errorf("create temp token file: %w", err)
	}

	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	cmd := exec.CommandContext(ctx, "chainctl", args...)
	cmd.Stdout = tmp
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		tmp.Close()
		return fmt.Errorf("chainctl failed to create token: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp token file: %w", err)
	}

	if err := os.Rename(tmp.Name(), dest); err != nil {
		return fmt.Errorf("failed to rename token file: %w", err)
	}

	return nil
}

func cleanupTokens(dir, pkg string) error {
	pkgDir := filepath.Join(dir, pkg)
	matches, err := filepath.Glob(filepath.Join(pkgDir, ".*.token"))
	if err != nil {
		return fmt.Errorf("failed to glob tokens: %w", err)
	}

	var errs []error
	for _, match := range matches {
		if err := os.Remove(match); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
