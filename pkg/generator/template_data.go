package generator

import "gitlab.crudus.no/crudus/swagger-codegen/pkg/codegen"

// GlobalData is the top-level data passed to supporting file templates.
type GlobalData struct {
	PubName        string
	PubVersion     string
	PubDescription string
	BrowserClient  bool
	BasePath       string
	AppVersion     string
	AppDescription string
	ApiDocPath     string
	ModelDocPath   string
	Models         []codegen.ModelData
	Apis           []codegen.ApiData
	AuthMethods    []codegen.AuthMethodData
	ApiInfo        *ApiInfo
}

type ApiInfo struct {
	Apis []codegen.ApiData
}

// ModelTemplateData is the data passed to model templates.
type ModelTemplateData struct {
	PubName string
	Models  []ModelWrapper
}

type ModelWrapper struct {
	Model codegen.ModelData
}

// ApiTemplateData is the data passed to API templates.
type ApiTemplateData struct {
	PubName    string
	Operations *codegen.ApiData
}
