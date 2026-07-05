package command

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mugon-net/cli/internal/model"
	"github.com/mugon-net/cli/internal/service"
	"github.com/mugon-net/cli/internal/templates"
	"github.com/urfave/cli/v3"
)

func watchSourceDir(ctx context.Context, sourceDir string, onChange func()) {
	modTimes := make(map[string]time.Time)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	initialScanDone := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			newModTimes := make(map[string]time.Time)
			filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error { //nolint:errcheck
				if err != nil || info.IsDir() {
					return nil
				}
				newModTimes[path] = info.ModTime()
				return nil
			})

			if initialScanDone {
				changed := false
				for path, mod := range newModTimes {
					if prev, ok := modTimes[path]; !ok || mod.After(prev) {
						changed = true
						break
					}
				}
				if !changed {
					for path := range modTimes {
						if _, ok := newModTimes[path]; !ok {
							changed = true
							break
						}
					}
				}
				if changed {
					onChange()
				}
			}

			modTimes = newModTimes
			initialScanDone = true
		}
	}
}

func findFreePort(startPort int) (net.Listener, int, error) {
	for port := startPort; port < startPort+100; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			return ln, port, nil
		}
	}
	return nil, 0, fmt.Errorf("no free port found in range %d-%d", startPort, startPort+99)
}

func ExecuteDevCommand(ctx context.Context, c *cli.Command) error {
	globalConfig := model.GetGlobalConfig(ctx)
	projectConfig := model.GetProjectConfig(ctx)

	mainLn, mainPort, err := findFreePort(3000)
	if err != nil {
		return err
	}

	childframeLn, gameFramePort, err := findFreePort(mainPort + 1)
	if err != nil {
		return err
	}

	parentframeFS, err := templates.ParentframeFileSystem(*projectConfig, mainPort, gameFramePort)
	if err != nil {
		return fmt.Errorf("failed to load parentframe: %w", err)
	}

	mainServer := &http.Server{Handler: http.FileServer(parentframeFS)}
	// TODO Add Middleware that checks the path of the served file and overwrites the csp meta tag for the index.html
	childframeHandler := http.FileServer(http.Dir(projectConfig.DistributionDir))
	childframeServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Allow-CSP-From", "*")
			childframeHandler.ServeHTTP(w, r)
		}),
	}

	ctx, stop := context.WithCancel(ctx)
	defer stop()

	var mu sync.Mutex
	var debounceTimer *time.Timer

	triggerBuild := func() {
		mu.Lock()
		defer mu.Unlock()
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(200*time.Millisecond, func() {
			commandConfig, err := projectConfig.GetCommand("build", "dev")
			if err != nil {
				fmt.Fprintf(os.Stderr, "build command not found: %v\n", err)
				return
			}
			if err := service.ExecuteProjectCommand(commandConfig, *globalConfig, *projectConfig); err != nil {
				fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
			}
		})
	}

	go watchSourceDir(ctx, projectConfig.SourceDir, triggerBuild)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mainServer.Shutdown(shutdownCtx)
		_ = childframeServer.Shutdown(shutdownCtx)
	}()

	errCh := make(chan error, 2)
	go func() {
		if err := mainServer.Serve(mainLn); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	go func() {
		if err := childframeServer.Serve(childframeLn); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	fmt.Printf("Dev server listening on http://localhost:%d\n", mainPort)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return nil
	}
}
