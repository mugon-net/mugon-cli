package model

type NoIndexJsPublishError struct {
}

func (NoIndexJsPublishError) Error() string {
	return "Publish failed. The distribution folder does not contain a index.js file."
}

type CommandNotFoundRunError struct {
}

func (CommandNotFoundRunError) Error() string {
	return "Command not found. Check mugon.toml if command with matching name, scope and os exists."
}

type DistributionFolderEmptyPublishError struct {
}

func (DistributionFolderEmptyPublishError) Error() string {
	return "Publish failed. The distribution folder is empty."
}

type NoProjectApiKeyFound struct {
}

func (NoProjectApiKeyFound) Error() string {
	return "No api key found for the current project. Use `mugon login` or set the `MUGON_PROJECT_API_KEY` environment variable to your api key."
}
