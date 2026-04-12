package command

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/mugon-net/cli/internal/model"
	"github.com/mugon-net/cli/internal/service"
	"github.com/urfave/cli/v3"
)

func ExecutePublishCommand(ctx context.Context, c *cli.Command) error {
	globalConfig := model.GetGlobalConfig(ctx)
	projectConfig := model.GetProjectConfig(ctx)

	var buildCommand *model.CommandConfig

	if command, err := projectConfig.GetCommand("build", "dev"); err == nil {
		buildCommand = &command
	}

	if command, err := projectConfig.GetCommand("build", model.DefaultCommandScope); buildCommand == nil && err == nil {
		buildCommand = &command
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
		if d.IsDir() {
			return nil
		}
		if err != nil {
			return nil
		}
		fileInfo, err := d.Info()
		if err != nil {
			return nil
		}

		filePaths = append(filePaths, filepath.Join(path, d.Name()))
		fileSizes = append(fileSizes, fileInfo.Size())
		return nil
	})
	if err != nil {
		return err
	}

	if len(filePaths) == 0 {
		return model.DistributionFolderEmptyPublishError{}
	}

	hasIndexJs := false
	for _, v := range filePaths {
		if v == "index.js" {
			hasIndexJs = true
		}
	}

	if !hasIndexJs {
		return model.NoIndexJsPublishError{}
	}

	api, err := service.InitApi(*globalConfig, *projectConfig)
	if err != nil {
		return err
	}

	game, err := api.GetGameByProjectName(ctx, projectConfig.Id)
	if err != nil {
		return err
	}

	gameVersionId, err := api.CreateVersion(ctx, game.Id, projectConfig.Version)
	if err != nil {
		return err
	}

	numChunks := len(filePaths) / 10

	for i := 0; i < numChunks; i++ {
		filePathChunk := filePaths[i*10 : (i+1)*10]
		fileSizeChunk := fileSizes[i*10 : (i+1)*10]
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
