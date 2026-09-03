package codegen

import (
	"os"
	"testing"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/swagger"
)

func TestModelRefsIn(t *testing.T) {
	known := map[string]bool{"Pet": true, "Tag": true, "Category": true}
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"String", nil},
		{"Pet", []string{"Pet"}},
		{"List<Pet>", []string{"Pet"}},
		{"Map<String, Pet>", []string{"Pet"}},
		{"List<Map<String, Tag>>", []string{"Tag"}},
		{"List<Pet>", []string{"Pet"}},
		{"Petunia", nil}, // must match whole token, not substring
	}
	for _, c := range cases {
		got := modelRefsIn(c.in, known)
		if len(got) != len(c.want) {
			t.Errorf("modelRefsIn(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("modelRefsIn(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestPruneUnusedModels_Synthetic(t *testing.T) {
	models := []ModelData{
		{Classname: "Pet", HasVars: true, Vars: []PropertyData{
			{Datatype: "Category", ComplexType: "Category"},
			{Datatype: "List<Tag>", Items: &ItemData{Datatype: "Tag"}, ComplexType: "Tag"},
		}},
		{Classname: "Category"},
		{Classname: "Tag"},
		{Classname: "Orphan"},      // referenced by nobody
		{Classname: "OrphanChild"}, // only referenced by Orphan
	}
	models[3].Vars = []PropertyData{{Datatype: "OrphanChild", ComplexType: "OrphanChild"}}

	apis := []ApiData{{Classname: "PetApi", Operations: []OperationData{
		{ReturnType: "List<Pet>", ReturnBaseType: "Pet"},
	}}}

	got := PruneUnusedModels(models, apis, nil)
	gotNames := map[string]bool{}
	for _, m := range got {
		gotNames[m.Classname] = true
	}

	for _, want := range []string{"Pet", "Category", "Tag"} {
		if !gotNames[want] {
			t.Errorf("expected %q to be kept, but it was pruned", want)
		}
	}
	for _, dropped := range []string{"Orphan", "OrphanChild"} {
		if gotNames[dropped] {
			t.Errorf("expected %q to be pruned, but it was kept", dropped)
		}
	}

	// keep should retain an unreferenced model AND its transitive dependencies.
	gotKeep := PruneUnusedModels(models, apis, []string{"Orphan", "Nonexistent"})
	keepNames := map[string]bool{}
	for _, m := range gotKeep {
		keepNames[m.Classname] = true
	}
	for _, want := range []string{"Orphan", "OrphanChild"} {
		if !keepNames[want] {
			t.Errorf("keep: expected %q to be kept, but it was pruned", want)
		}
	}
}

func TestPruneUnusedModels_KeepsAliasTargets(t *testing.T) {
	definitions := map[string]*swagger.Schema{
		"DeletedAt": {Ref: "#/definitions/NullTime"},
		"NullTime": {Type: "object", Properties: map[string]*swagger.Schema{
			"Time":  {Type: "string", Format: "date-time"},
			"Valid": {Type: "boolean"},
		}},
		"Orphan": {Type: "object"},
	}
	models := BuildModels(definitions, definitions, true, []string{"DeletedAt", "NullTime", "Orphan"}, nil)
	if models[0].DataType != "NullTime" {
		t.Fatalf("DeletedAt underlying type = %q, want NullTime", models[0].DataType)
	}
	apis := []ApiData{{Classname: "RecordApi", Operations: []OperationData{
		{ReturnType: "DeletedAt", ReturnBaseType: "DeletedAt"},
	}}}

	got := PruneUnusedModels(models, apis, nil)
	gotNames := make(map[string]bool, len(got))
	for _, model := range got {
		gotNames[model.Classname] = true
	}

	for _, want := range []string{"DeletedAt", "NullTime"} {
		if !gotNames[want] {
			t.Errorf("expected %q to be kept, but it was pruned", want)
		}
	}
	if gotNames["Orphan"] {
		t.Error("expected Orphan to be pruned")
	}
}

// TestPruneUnusedModels_Testdata verifies against the real spec that pruning
// keeps every model reachable from the API surface and drops the rest, and that
// no kept model references a pruned model (closure is complete).
func TestPruneUnusedModels_Testdata(t *testing.T) {
	data, err := os.ReadFile("../../testdata/swagger.json")
	if err != nil {
		t.Skipf("testdata not available: %v", err)
	}
	spec, err := swagger.Parse(data)
	if err != nil {
		t.Fatalf("parsing spec: %v", err)
	}

	models := BuildModels(spec.Definitions, spec.Definitions, true, spec.DefinitionOrder, nil)
	apis := BuildApis(spec)

	pruned := PruneUnusedModels(models, apis, nil)

	if len(pruned) == 0 {
		t.Fatal("pruning removed all models")
	}
	if len(pruned) > len(models) {
		t.Fatalf("pruned set (%d) larger than full set (%d)", len(pruned), len(models))
	}

	kept := make(map[string]bool, len(pruned))
	for _, m := range pruned {
		kept[m.Classname] = true
	}
	knownAll := make(map[string]bool, len(models))
	for _, m := range models {
		knownAll[m.Classname] = true
	}

	// Closure check: every model reference inside a kept model must itself be kept.
	for _, m := range pruned {
		for _, ref := range modelRefsIn(m.DataType, knownAll) {
			if !kept[ref] {
				t.Errorf("kept model %q references pruned model %q via its underlying type",
					m.Classname, ref)
			}
		}
		for _, v := range m.Vars {
			for _, ref := range modelRefsIn(v.Datatype, knownAll) {
				if !kept[ref] {
					t.Errorf("kept model %q references pruned model %q via property %q",
						m.Classname, ref, v.BaseName)
				}
			}
		}
	}

	t.Logf("pruned %d/%d models down to %d", len(models)-len(pruned), len(models), len(pruned))
}
