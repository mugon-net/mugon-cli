package model

type NoIndexHtmlPublishError struct {
}

func (NoIndexHtmlPublishError) Error() string {
	return "Publish failed. The distribution folder does not contain an index.html file."
}

type NoSdkBundledPublishError struct {
}

func (NoSdkBundledPublishError) Error() string {
	return "Publish failed. No @mugon/sdk was found in the distribution folder: the game must bundle the SDK (or ship mugon.iife.js) and load it before anything else."
}

type SdkMajorMismatchPublishError struct {
	Declared string
	Bundled  string
}

func (err SdkMajorMismatchPublishError) Error() string {
	return "Publish failed. mugon.toml declares js-sdk-version " + err.Declared + " but the distribution folder bundles @mugon/sdk " + err.Bundled + ". The major versions must match."
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
