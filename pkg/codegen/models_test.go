package codegen

import (
	"testing"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/swagger"
)

// A definition that is only a $ref must become an alias of its target, not an
// empty class: DeletedAt -> NullTime, or a response declared as an alias of a
// model (FormFamnProductResponse -> FormFamnProduct).
func TestBuildModel_RefOnlyDefinitionIsAlias(t *testing.T) {
	definitions := map[string]*swagger.Schema{
		"NullTime": {Type: "object", Properties: map[string]*swagger.Schema{
			"Time":  {Type: "string", Format: "date-time"},
			"Valid": {Type: "boolean"},
		}},
		"DeletedAt": {Ref: "#/definitions/NullTime"},
	}
	model := buildModel("DeletedAt", definitions["DeletedAt"], definitions, false, nil)
	if !model.IsRefAlias {
		t.Fatalf("expected DeletedAt to be a ref alias, got %+v", model)
	}
	if model.DataType != "NullTime" {
		t.Errorf("DataType = %q, want NullTime", model.DataType)
	}
	if model.IsAlias || model.IsListAlias || model.IsMapAlias || model.HasVars || model.IsEnum {
		t.Errorf("ref alias must not be any other kind: %+v", model)
	}

	// The target itself stays a class.
	target := buildModel("NullTime", definitions["NullTime"], definitions, false, nil)
	if target.IsRefAlias || !target.HasVars {
		t.Errorf("NullTime should be a class with vars, got %+v", target)
	}

	// A configured native enum wins over the alias classification.
	enum := buildModel("DeletedAt", definitions["DeletedAt"], definitions, false,
		map[string][]string{"DeletedAt": {"a", "b"}})
	if enum.IsRefAlias || !enum.IsNativeEnum {
		t.Errorf("native enum config must override the alias, got %+v", enum)
	}
}

// The alias must keep its target reachable when unused models are pruned.
func TestPruneUnusedModels_KeepsRefAliasTarget(t *testing.T) {
	models := []ModelData{
		{Classname: "Pet", HasVars: true, Vars: []PropertyData{
			{Datatype: "DeletedAt", ComplexType: "DeletedAt"},
		}},
		{Classname: "DeletedAt", IsRefAlias: true, DataType: "NullTime"},
		{Classname: "NullTime", HasVars: true},
	}
	apis := []ApiData{{Classname: "PetApi", Operations: []OperationData{
		{ReturnType: "Pet", ReturnBaseType: "Pet"},
	}}}
	kept := map[string]bool{}
	for _, m := range PruneUnusedModels(models, apis, nil) {
		kept[m.Classname] = true
	}
	for _, want := range []string{"Pet", "DeletedAt", "NullTime"} {
		if !kept[want] {
			t.Errorf("expected %q to be kept", want)
		}
	}
}
