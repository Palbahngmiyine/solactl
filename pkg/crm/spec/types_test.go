package spec

import (
	"encoding/json"
	"testing"
)

func TestPathItemUnmarshal_MergesPathLevelParameters(t *testing.T) {
	raw := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "crm", "version": "1"},
		"paths": {
			"/crm-core/v1/records/{id}": {
				"summary": "path item metadata",
				"parameters": [
					{"name": "id", "in": "path", "required": true},
					{"name": "includeDeleted", "in": "query"}
				],
				"get": {
					"summary": "get record",
					"parameters": [
						{"name": "limit", "in": "query"}
					]
				}
			}
		}
	}`)

	var spec OpenApiSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	cmds := MapSpec(&spec)
	if len(cmds) != 1 {
		t.Fatalf("want 1 command, got %#v", cmds)
	}
	cmd := cmds[0]
	if len(cmd.PathParams) != 1 || cmd.PathParams[0].Name != "id" {
		t.Fatalf("path-level path param not mapped: %#v", cmd.PathParams)
	}
	if len(cmd.QueryParams) != 2 {
		t.Fatalf("want 2 query params, got %#v", cmd.QueryParams)
	}
	gotNames := map[string]bool{}
	for _, param := range cmd.QueryParams {
		gotNames[param.Name] = true
	}
	if !gotNames["includeDeleted"] || !gotNames["limit"] {
		t.Fatalf("query params not merged: %#v", cmd.QueryParams)
	}
}

func TestPathItemUnmarshal_OperationParameterOverridesPathLevel(t *testing.T) {
	raw := []byte(`{
		"parameters": [
			{"name": "limit", "in": "query", "description": "path-level"}
		],
		"get": {
			"summary": "list",
			"parameters": [
				{"name": "limit", "in": "query", "description": "operation-level"}
			]
		}
	}`)

	var item PathItem
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	params := item["get"].Parameters
	if len(params) != 1 {
		t.Fatalf("want one overridden param, got %#v", params)
	}
	if params[0].Description != "operation-level" {
		t.Fatalf("operation param should override path-level param, got %#v", params[0])
	}
}
