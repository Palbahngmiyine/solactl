package spec

import (
	"testing"
)

func TestMapSpec_BasicPatterns(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		method         string
		op             OperationObject
		wantResource   string
		wantAction     string
		wantPathParams int
	}{
		{"GET list", "/crm-core/v1/records", "get", OperationObject{Summary: "list"}, "records", "list", 0},
		{"GET get by id", "/crm-core/v1/records/{id}", "get", OperationObject{Summary: "get"}, "records", "get", 0},
		{"GET sub list", "/crm-core/v1/records/{id}/logs", "get", OperationObject{Summary: "logs"}, "records", "list-logs", 0},
		{"GET trash", "/crm-core/v1/records/trash", "get", OperationObject{Summary: "trash"}, "records", "list-trash", 0},
		{"GET search/fulltext/{id}", "/crm-core/v1/records/search/fulltext/{id}", "get", OperationObject{Summary: "fts"}, "records", "list-search-fulltext", 0},
		{"POST create", "/crm-core/v1/records", "post", OperationObject{Summary: "create"}, "records", "create", 0},
		{"POST bulk-delete", "/crm-core/v1/records/bulk/delete", "post", OperationObject{Summary: "bulk"}, "records", "bulk-delete", 0},
		{"POST restore", "/crm-core/v1/records/{id}/restore", "post", OperationObject{Summary: "restore"}, "records", "restore", 0},
		{"PUT replace", "/crm-core/v1/records/{id}", "put", OperationObject{Summary: "replace"}, "records", "replace", 0},
		{"DELETE delete", "/crm-core/v1/records/{id}", "delete", OperationObject{Summary: "delete"}, "records", "delete", 0},
		{"PATCH update", "/crm-core/v1/records/{id}", "patch", OperationObject{Summary: "patch"}, "records", "update", 0},
		{"PATCH operationId verb_name", "/crm-core/v1/records/{id}", "patch", OperationObject{Summary: "patch", OperationID: "patch_renameRecord"}, "records", "renameRecord", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := &OpenApiSpec{Paths: map[string]PathItem{tc.path: {tc.method: tc.op}}}
			cmds := MapSpec(spec)
			if len(cmds) != 1 {
				t.Fatalf("want 1 cmd, got %d", len(cmds))
			}
			cmd := cmds[0]
			if cmd.Resource != tc.wantResource {
				t.Errorf("resource: want %q got %q", tc.wantResource, cmd.Resource)
			}
			if cmd.Action != tc.wantAction {
				t.Errorf("action: want %q got %q", tc.wantAction, cmd.Action)
			}
		})
	}
}

func TestMapSpec_SkipsNonPrefixPaths(t *testing.T) {
	spec := &OpenApiSpec{Paths: map[string]PathItem{
		"/messages/v4/messages": {"get": OperationObject{Summary: "x"}},
		"/crm-core/v1/records":  {"get": OperationObject{Summary: "x"}},
	}}
	cmds := MapSpec(spec)
	if len(cmds) != 1 || cmds[0].Resource != "records" {
		t.Fatalf("expected only the crm-core path mapped, got %#v", cmds)
	}
}

func TestMapSpec_DuplicateActionsDisambiguated(t *testing.T) {
	spec := &OpenApiSpec{Paths: map[string]PathItem{
		"/crm-core/v1/records":              {"get": OperationObject{Summary: "list"}},
		"/crm-core/v1/records/recent":       {"get": OperationObject{Summary: "recent"}},
		"/crm-core/v1/records/{id}":         {"get": OperationObject{Summary: "get"}},
		"/crm-core/v1/records/{id}/logs":    {"get": OperationObject{Summary: "logs"}},
		"/crm-core/v1/records/{id}/history": {"get": OperationObject{Summary: "history"}},
	}}
	cmds := MapSpec(spec)
	actions := make(map[string]int)
	for _, c := range cmds {
		actions[c.Action]++
	}
	for action, count := range actions {
		if count != 1 {
			t.Errorf("action %q duplicated %d times", action, count)
		}
	}
}

func TestMapSpec_DuplicateActionFallsBackToCounter(t *testing.T) {
	// Two operations on the same path-with-only-id, same method → same
	// extracted suffix is empty → counter should kick in.
	spec := &OpenApiSpec{Paths: map[string]PathItem{
		"/crm-core/v1/records/{id}":   {"patch": OperationObject{Summary: "a"}},
		"/crm-core/v1/records/{name}": {"patch": OperationObject{Summary: "b"}},
	}}
	cmds := MapSpec(spec)
	if len(cmds) != 2 {
		t.Fatalf("want 2 cmds, got %d", len(cmds))
	}
	a, b := cmds[0].Action, cmds[1].Action
	if a == b {
		t.Errorf("duplicate-action suffixing failed; both %q", a)
	}
	// One of them must be "update", the other suffixed.
	if a != "update" && b != "update" {
		t.Errorf("expected one to remain 'update'; got %q and %q", a, b)
	}
}

func TestMapSpec_PathQuerySplit(t *testing.T) {
	spec := &OpenApiSpec{Paths: map[string]PathItem{
		"/crm-core/v1/records/{id}/related/{related}": {
			"get": OperationObject{
				Summary: "related",
				Parameters: []ParameterObject{
					{Name: "id", In: "path", Required: true},
					{Name: "related", In: "path", Required: true},
					{Name: "limit", In: "query"},
					{Name: "X-Trace", In: "header"},
				},
			},
		},
	}}
	cmds := MapSpec(spec)
	if len(cmds) != 1 {
		t.Fatalf("want 1 cmd, got %d", len(cmds))
	}
	c := cmds[0]
	if len(c.PathParams) != 2 {
		t.Errorf("pathParams: want 2 got %d", len(c.PathParams))
	}
	if len(c.QueryParams) != 1 || c.QueryParams[0].Name != "limit" {
		t.Errorf("queryParams: want [limit] got %#v", c.QueryParams)
	}
	// header parameters intentionally dropped.
}

func TestMapSpec_BodyFlags(t *testing.T) {
	spec := &OpenApiSpec{Paths: map[string]PathItem{
		"/crm-core/v1/records": {
			"post": OperationObject{
				Summary:     "c",
				RequestBody: &RequestBodyObject{Required: true},
			},
		},
	}}
	cmds := MapSpec(spec)
	if !cmds[0].HasBody || !cmds[0].BodyRequired {
		t.Errorf("body flags wrong: %+v", cmds[0])
	}
}

func TestMapSpec_EmptyAndNilSafety(t *testing.T) {
	if got := MapSpec(nil); got != nil {
		t.Errorf("nil spec → want nil, got %#v", got)
	}
	if got := MapSpec(&OpenApiSpec{}); got != nil {
		t.Errorf("empty spec → want nil, got %#v", got)
	}
	// Path with empty resource segment must be skipped.
	spec := &OpenApiSpec{Paths: map[string]PathItem{"/crm-core/v1/": {"get": OperationObject{Summary: "x"}}}}
	if got := MapSpec(spec); len(got) != 0 {
		t.Errorf("empty resource → want 0 cmds, got %#v", got)
	}
}

func TestMapSpec_InvalidOperationSkipped(t *testing.T) {
	// PathItem entry that didn't decode into a real OperationObject (zero value)
	// must be skipped without producing a phantom command.
	spec := &OpenApiSpec{Paths: map[string]PathItem{
		"/crm-core/v1/records": {"get": OperationObject{}},
	}}
	cmds := MapSpec(spec)
	if len(cmds) != 0 {
		t.Errorf("zero-valued op should be skipped, got %#v", cmds)
	}
}

func TestResources(t *testing.T) {
	cmds := []MappedCommand{
		{Resource: "records"},
		{Resource: "entities"},
		{Resource: "records"},
		{Resource: "automations"},
	}
	got := Resources(cmds)
	want := []string{"automations", "entities", "records"}
	if len(got) != len(want) {
		t.Fatalf("want %v got %v", want, got)
	}
	for i, r := range want {
		if got[i] != r {
			t.Errorf("[%d] want %q got %q", i, r, got[i])
		}
	}
}

func TestCommandsForResource(t *testing.T) {
	cmds := []MappedCommand{
		{Resource: "records", Action: "list"},
		{Resource: "entities", Action: "list"},
		{Resource: "records", Action: "create"},
	}
	got := CommandsForResource(cmds, "records")
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].Action != "list" || got[1].Action != "create" {
		t.Errorf("order broken: %#v", got)
	}
}
