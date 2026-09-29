package generator

import (
	"strings"
	"testing"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/codegen"
)

func TestDartDiagnosticCredentialsAreRedacted(t *testing.T) {
	for _, name := range []string{"token", "accessToken", "refresh_token", "id-token", "password", "secret", "clientSecret", "api_key", "Authorization"} {
		t.Run(name, func(t *testing.T) {
			property := codegen.PropertyData{Name: "credential", BaseName: name, Datatype: "String"}
			got := renderToStringVars([]codegen.PropertyData{property})
			if got != "credential=[redacted], " {
				t.Fatalf("diagnostic exposes credential: %s", got)
			}
			// Redaction is diagnostic-only: keep the wire field name and value.
			got = renderFromJsonBody([]codegen.PropertyData{property})
			if !strings.Contains(strings.Join(strings.Fields(got), ""), "credential=json['"+name+"']") {
				t.Fatalf("wire decoding changed: %s", got)
			}
		})
	}
	got := renderToStringVars([]codegen.PropertyData{{Name: "requestId", BaseName: "requestId"}})
	if got != "requestId=$requestId, " {
		t.Fatalf("ordinary diagnostic changed: %s", got)
	}
}
