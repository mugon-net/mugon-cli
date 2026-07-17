package main

// The go generate command will NOT work if you cloned this repository from github
// It is only intended to be used in the internal mugon repository.
// All files that are output by this command are committed in the public repository.

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config .oapi-cfg.yml ../../specs/api/api-spec.yml
//go:generate npm --prefix internal/templates/parentframe install
//go:generate npm --prefix internal/templates/parentframe run build
