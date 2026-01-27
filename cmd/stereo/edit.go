package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func editCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit package manifest",
		Short: "Open a package manifest in your editor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg := strings.TrimSuffix(args[0], ".yaml")
			if pkg == "" {
				return fmt.Errorf("package name is required")
			}

			dir, err := findPackage(pkg)
			if err != nil {
				return fmt.Errorf("finding package %q: %w", args[0], err)
			}

			manifestPath := filepath.Join(dir, pkg) + ".yaml"

			editorPath, editorArgs, err := resolveEditor()
			if err != nil {
				return err
			}

			editorCmd := exec.CommandContext(cmd.Context(), editorPath, append(editorArgs, manifestPath)...)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr
			return editorCmd.Run()
		},
	}

	return cmd
}

func resolveEditor() (string, []string, error) {
	if editor := strings.TrimSpace(os.Getenv("STEREO_EDITOR")); editor != "" {
		return parseEditor(editor)
	}

	if editor := strings.TrimSpace(os.Getenv("EDITOR")); editor != "" {
		return parseEditor(editor)
	}

	path, args, err := parseEditor("vi")
	if err != nil {
		return "", nil, fmt.Errorf("editor not found; set STEREO_EDITOR or EDITOR in your environment")
	}

	return path, args, nil
}

func parseEditor(editor string) (string, []string, error) {
	fields := strings.Fields(editor)
	if len(fields) == 0 {
		return "", nil, fmt.Errorf("editor value is empty")
	}

	path, err := exec.LookPath(fields[0])
	if err != nil {
		return "", nil, fmt.Errorf("editor %q not found in PATH", fields[0])
	}

	return path, fields[1:], nil
}
