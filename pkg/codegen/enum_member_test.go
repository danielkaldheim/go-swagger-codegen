package codegen

import "testing"

func TestToEnumMemberName(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"plain value", "inventory", "inventory"},
		{"snake case", "valuable_asset", "valuableAsset"},
		{"kebab case", "valuable-asset", "valuableAsset"},
		{"dart keyword", "new", "new_"},
		// Keyword detection is case-insensitive, so an odd-cased wire value is
		// still escaped rather than emitted as a bare keyword.
		{"keyword in another case", "IS", "iS_"},
		{"enum member collision", "index", "index_"},
		{"object member collision", "hashCode", "hashCode_"},
		{"leading digit", "4wd", "v4wd"},
		{"empty", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := toEnumMemberName(tc.value); got != tc.want {
				t.Fatalf("toEnumMemberName(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

// The escaped member must still round-trip through the generated fromJson and
// toJson switches, which key off WireValue rather than the Dart name.
func TestBuildNativeEnumMembersKeepsWireValue(t *testing.T) {
	members := buildNativeEnumMembers([]string{"new", "used"}, nil)

	if len(members) != 3 {
		t.Fatalf("expected unknown + 2 members, got %d", len(members))
	}
	if members[0].Name != NativeEnumUnknownMember {
		t.Fatalf("expected unknown first, got %q", members[0].Name)
	}
	if members[1].Name != "new_" || members[1].WireValue != "new" {
		t.Fatalf("escaped member lost its wire value: %+v", members[1])
	}
}
