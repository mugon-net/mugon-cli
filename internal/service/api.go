package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/mugon-net/cli/internal/model"
	"github.com/mugon-net/cli/internal/oapi"
)

type Api struct {
	client *oapi.ClientWithResponses
}

func InitApi(globalConfig model.GlobalConfig, projectConfig model.ProjectConfig) (*Api, error) {
	serverUrl := model.DefaultServerUrl
	if len(strings.TrimSpace(globalConfig.MugonNetApiUrlOverride)) != 0 {
		serverUrl = globalConfig.MugonNetApiUrlOverride
	}

	apiKey, err := model.GetApiKey(globalConfig, projectConfig)
	if err != nil {
		return nil, err
	}

	client, err := oapi.NewClientWithResponses(serverUrl, oapi.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add("api-key", apiKey)
		return nil
	}))
	if err != nil {
		return nil, err
	}
	return &Api{
		client: client,
	}, nil
}
func (api *Api) GetGameByProjectName(ctx context.Context, projectName string) (oapi.GameDTO, error) {
	response, err := api.client.GetGameLatestWithResponse(ctx, &oapi.GetGameLatestParams{GameName: projectName})
	if err != nil {
		return oapi.GameDTO{}, err
	}
	if response.HTTPResponse.StatusCode != 200 {
		return oapi.GameDTO{}, fmt.Errorf("Failed to create version: %s", response.HTTPResponse.Status)
	}
	return response.JSON200.Game, nil
}

func (api *Api) CreateVersion(ctx context.Context, projectId uuid.UUID, version string) (uuid.UUID, error) {
	response, err := api.client.CreateVersionWithResponse(ctx, oapi.CreateVersionRequest{GameId: projectId, Version: version})
	if err != nil {
		return uuid.UUID{}, err
	}
	if response.HTTPResponse.StatusCode != 200 {
		return uuid.UUID{}, fmt.Errorf("Failed to create version: %s", response.HTTPResponse.Status)
	}
	return response.JSON200.GameVersion.Id, nil
}

func (api *Api) DeleteVersion(ctx context.Context, versionId uuid.UUID) error {
	response, err := api.client.DeleteGameVersionWithResponse(ctx, &oapi.DeleteGameVersionParams{GameVersionId: versionId})
	if err != nil {
		return err
	}
	if response.HTTPResponse.StatusCode != 200 {
		return fmt.Errorf("Failed to delete version: %s", response.HTTPResponse.Status)
	}
	return nil
}

func (api *Api) RequestVersionFileUploadUrls(ctx context.Context, versionId uuid.UUID, filePaths []string, fileSizes []int64) (map[string]oapi.UploadUrlDTO, error) {
	fileMetadata := make([]oapi.FileUploadMetadataDTO, 0)
	for i := range filePaths {
		fileMetadata = append(fileMetadata, oapi.FileUploadMetadataDTO{Path: filePaths[i], Size: fileSizes[i]})
	}

	response, err := api.client.GetGameVersionUploadUrlsWithResponse(ctx, oapi.GetGameVersionUploadUrlsRequest{GameVersionId: versionId, FileMetadata: fileMetadata})
	if err != nil {
		return nil, err
	}
	if response.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to request upload urls for version: %s", response.HTTPResponse.Status)
	}

	fileUploadUrlMap := make(map[string]oapi.UploadUrlDTO)
	for i := range filePaths {
		fileUploadUrlMap[filePaths[i]] = response.JSON200.UploadUrls[i]
	}

	return fileUploadUrlMap, nil
}

func (api *Api) UploadVersionFile(ctx context.Context, filePath string, uploadUrlDTO oapi.UploadUrlDTO) error {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create a buffer to store our multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add form fields from the DTO
	for key, value := range uploadUrlDTO.FormData {
		part, err := writer.CreateFormField(key)
		if err != nil {
			return fmt.Errorf("failed to create form field: %w", err)
		}
		_, err = io.WriteString(part, value)
		if err != nil {
			return fmt.Errorf("failed to write form field: %w", err)
		}
	}

	// Add the file to the form
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	// Close the writer to finalize the multipart form
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "POST", uploadUrlDTO.Url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set the content type header
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Create a client and execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("upload failed with status: %s", resp.Status)
	}

	return nil
}
func (api *Api) FinishVersionUpload(ctx context.Context, versionId uuid.UUID) error {
	visibility := oapi.GameVisibilityEnumINTERNAL
	response, err := api.client.UpdateGameVersionWithResponse(ctx, oapi.UpdateGameVersionRequest{Id: versionId, Visibility: &visibility})
	if err != nil {
		return err
	}
	if response.HTTPResponse.StatusCode != 200 {
		return fmt.Errorf("failed to create version: %s", response.HTTPResponse.Status)
	}
	return nil
}
