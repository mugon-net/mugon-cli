package main

func main() {
	// commands:
	// - login
	//      Enter api key, authenticate & save credentials locally (maybe also roles? maybe later)
	//          Also needs options to directly pass credentials to be able to be used in ci/cd
	// - init
	//      Init a new game project in the cwd, creates a mugon.yaml file
	//          A bit like .firebaserc, includes output folder, configs, game id, js-sdk version, ...
	//          Also allow user to select templates: empty, typescript, bevy
	// - publish
	//      Publish current files in the output folder as a new version
	//          1. Creates the new (preliminary) version via the api
	//          Repeat until all files uploaded:
	//              2. Retrieves up to 10 presigned urls at a time to upload them
	//              3. Uploads the files using
	//          4. "Finish" the version creation, backend does a couple of checks, removes "preliminary" status
	//              Maybe copies the files to the serving bucket location? Should they still be seperated?
	//              Probably, so auto clean up can take care of half uploaded version files?
}
