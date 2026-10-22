package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/mugon-net/cli/internal/model"
	"github.com/urfave/cli/v3"
)

const editorBinaryName = "mugon-editor"

// ExecuteEditorCommand runs the mugon-editor binary with all arguments passed through verbatim. Without arguments
// mugon-editor opens its scene editor.
func ExecuteEditorCommand(ctx context.Context, c *cli.Command) error {
	home, _ := os.UserHomeDir()
	path, err := findEditor(exec.LookPath, home)
	if err != nil {
		return err
	}

	editor := exec.CommandContext(ctx, path, c.Args().Slice()...)
	editor.Stdin = os.Stdin
	editor.Stdout = os.Stdout
	editor.Stderr = os.Stderr
	if err := editor.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return cli.Exit("", exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run %s: %w", path, err)
	}
	return nil
}

// findEditor looks for the mugon-editor binary on PATH, then in the folder the install scripts use.
func findEditor(lookPath func(string) (string, error), home string) (string, error) {
	if path, err := lookPath(editorBinaryName); err == nil {
		return path, nil
	}

	fileName := editorBinaryName
	if runtime.GOOS == "windows" {
		fileName += ".exe"
	}
	installDir := filepath.Join(home, ".mugon", "editor")
	installed := filepath.Join(installDir, fileName)
	if info, err := os.Stat(installed); err == nil && !info.IsDir() {
		return installed, nil
	}

	return "", model.EditorNotFoundError{InstallDir: installDir}
}
