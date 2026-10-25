package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/mugon-net/cli/internal/editorinstall"
	"github.com/mugon-net/cli/internal/model"
	"github.com/mugon-net/cli/internal/version"
	"github.com/urfave/cli/v3"
)

const editorBinaryName = "mugon-editor"

const toolsURLEnv = "MUGON_TOOLS_URL"

// ExecuteEditorCommand manages the editor installation for `install`, `update` and `uninstall`; everything else runs
// the mugon-editor binary with all arguments passed through verbatim. Without arguments mugon-editor opens its
// scene editor.
func ExecuteEditorCommand(ctx context.Context, c *cli.Command) error {
	home, _ := os.UserHomeDir()
	args := c.Args().Slice()
	if len(args) > 0 {
		switch args[0] {
		case "install", "update", "uninstall":
			return manageEditor(args[0], args[1:], home)
		}
	}
	path, err := findEditor(exec.LookPath, home)
	if err != nil {
		return err
	}

	editor := exec.CommandContext(ctx, path, args...)
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

func manageEditor(action string, args []string, home string) error {
	requested, all, err := parseManageArgs(action, args)
	if err != nil {
		return err
	}
	installer := editorinstall.NewInstaller(home, os.Getenv(toolsURLEnv), version.Version, os.Stdout)
	switch action {
	case "install":
		_, err = installer.Install(requested)
	case "update":
		_, err = installer.Update()
	default:
		err = installer.Uninstall(requested, all)
	}
	return err
}

// parseManageArgs accepts `--version X` (install, uninstall) and `--all` (uninstall).
func parseManageArgs(action string, args []string) (requested string, all bool, err error) {
	for position := 0; position < len(args); position++ {
		switch args[position] {
		case "--version", "-v":
			if action == "update" || position+1 >= len(args) {
				return "", false, fmt.Errorf("usage: mugon editor %s", manageUsage(action))
			}
			position++
			requested = args[position]
		case "--all":
			if action != "uninstall" {
				return "", false, fmt.Errorf("usage: mugon editor %s", manageUsage(action))
			}
			all = true
		default:
			return "", false, fmt.Errorf("unknown argument %q; usage: mugon editor %s", args[position], manageUsage(action))
		}
	}
	return requested, all, nil
}

func manageUsage(action string) string {
	switch action {
	case "install":
		return "install [--version X.Y.Z]"
	case "update":
		return "update"
	}
	return "uninstall [--version X.Y.Z | --all]"
}

// findEditor looks for the mugon-editor binary on PATH, then in the active version of the managed install.
func findEditor(lookPath func(string) (string, error), home string) (string, error) {
	if path, err := lookPath(editorBinaryName); err == nil {
		return path, nil
	}

	fileName := editorBinaryName
	if runtime.GOOS == "windows" {
		fileName += ".exe"
	}
	installDir := filepath.Join(home, ".mugon", "editor")
	installer := editorinstall.Installer{Root: installDir}
	if current := installer.Current(); current != "" {
		installed := filepath.Join(installDir, current, fileName)
		if info, err := os.Stat(installed); err == nil && !info.IsDir() {
			return installed, nil
		}
	}

	return "", model.EditorNotFoundError{InstallDir: installDir}
}
