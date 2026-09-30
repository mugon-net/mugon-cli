package command

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mugon-net/cli/internal/model"
	"github.com/mugon-net/cli/internal/service"
	"github.com/urfave/cli/v3"
)

func ExecutePublishCommand(ctx context.Context, c *cli.Command) error {
	globalConfig := model.GetGlobalConfig(ctx)
	projectConfig := model.GetProjectConfig(ctx)

	var buildCommand *model.CommandConfig

	if command, err := projectConfig.GetCommand("build", model.DefaultCommandScope); err == nil {
		buildCommand = &command
	}

	if buildCommand == nil {
		if command, err := projectConfig.GetCommand("build", "dev"); err == nil {
			buildCommand = &command
		}
	}

	if buildCommand != nil {
		err := service.ExecuteProjectCommand(*buildCommand, *globalConfig, *projectConfig)
		if err != nil {
			return err
		}
	} else {
		fmt.Printf("Warning: No build command found")
	}

	filePaths := make([]string, 0)
	fileSizes := make([]int64, 0)
	err := filepath.WalkDir(projectConfig.DistributionDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		fileInfo, err := d.Info()
		if err != nil {
			return nil
		}
		relativePath, err := filepath.Rel(projectConfig.DistributionDir, path)
		if err != nil {
			return nil
		}

		filePaths = append(filePaths, filepath.ToSlash(relativePath))
		fileSizes = append(fileSizes, fileInfo.Size())
		return nil
	})
	if err != nil {
		return err
	}

	if len(filePaths) == 0 {
		return model.DistributionFolderEmptyPublishError{}
	}

	if !slices.Contains(filePaths, "index.html") {
		return model.NoIndexHtmlPublishError{}
	}

	jsSdkMajor, err := projectConfig.JsSdkMajor()
	if err != nil {
		return fmt.Errorf("invalid js-sdk-version %q in mugon.toml: %w", projectConfig.JsSdkVersion, err)
	}
	if err := checkBundledSdk(*projectConfig, jsSdkMajor); err != nil {
		return err
	}

	api, err := service.InitApi(*globalConfig, *projectConfig)
	if err != nil {
		return err
	}

	game, err := api.GetGameByProjectName(ctx, projectConfig.Id)
	if err != nil {
		return err
	}

	gameVersionId, err := api.CreateVersion(ctx, game.Id, projectConfig.Version, jsSdkMajor)
	if err != nil {
		return err
	}

	for i := 0; i < len(filePaths); i += 10 {
		end := i + 10
		if end > len(filePaths) {
			end = len(filePaths)
		}
		filePathChunk := filePaths[i:end]
		fileSizeChunk := fileSizes[i:end]
		uploadUrls, err := api.RequestVersionFileUploadUrls(ctx, gameVersionId, filePathChunk, fileSizeChunk)
		if err != nil {
			_ = api.DeleteVersion(ctx, gameVersionId)
			return err
		}
		for filePath, fileUploadUrl := range uploadUrls {
			err = api.UploadVersionFile(ctx, filePath, fileUploadUrl)
			if err != nil {
				_ = api.DeleteVersion(ctx, gameVersionId)
				return fmt.Errorf("failed to upload file: '%s'", err)
			}
		}
	}
	err = api.FinishVersionUpload(ctx, gameVersionId)
	if err != nil {
		_ = api.DeleteVersion(ctx, gameVersionId)
		return err
	}

	return nil
}

func checkBundledSdk(projectConfig model.ProjectConfig, jsSdkMajor int) error {
	bundledVersions, err := service.FindSdkVersions(projectConfig.DistributionDir)
	if err != nil {
		return err
	}
	if len(bundledVersions) == 0 {
		return model.NoSdkBundledPublishError{}
	}
	for _, bundled := range bundledVersions {
		bundledMajor, _, _ := strings.Cut(bundled, ".")
		if bundledMajor != strconv.Itoa(jsSdkMajor) {
			return model.SdkMajorMismatchPublishError{Declared: projectConfig.JsSdkVersion, Bundled: bundled}
		}
		if bundled != projectConfig.JsSdkVersion {
			fmt.Printf("Warning: mugon.toml declares js-sdk-version %s but the game bundles %s\n", projectConfig.JsSdkVersion, bundled)
		}
	}
	if !service.SdkLoadedFirst(projectConfig.DistributionDir) {
		fmt.Printf("Warning: the first <script> of index.html does not contain the mugon SDK. It must load before anything else, or large files will not be cached.\n")
	}
	return nil
}
