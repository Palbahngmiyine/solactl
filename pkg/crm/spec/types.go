// Package spec is the CRM OpenAPI loader, on-disk cache, and command mapper.
//
// The implementation mirrors @solapi/crm-cli (sdk/cli/src/spec/*.ts) so that
// solactl exposes the same dynamic `<resource> <action>` tree as the upstream
// node CLI. See docs/crm-cli-spec.md.
package spec

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OpenApiSpec is the minimal subset of OpenAPI 3.x that the CRM CLI consumes.
// Fields not used by mapper/loader are kept as raw maps so backend additions
// do not break decoding.
type OpenApiSpec struct {
	OpenAPI string              `json:"openapi"`
	Info    SpecInfo            `json:"info"`
	Paths   map[string]PathItem `json:"paths"`
	Comp    map[string]any      `json:"components,omitempty"`
}

// SpecInfo is the `info` block.
type SpecInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// PathItem holds operations keyed by lowercase HTTP method (get/post/...).
// Non-operation keys are ignored while path-level parameters are merged into
// each operation so standard OpenAPI PathItem objects decode successfully.
type PathItem map[string]OperationObject

var supportedPathItemMethods = map[string]struct{}{
	"delete": {},
	"get":    {},
	"patch":  {},
	"post":   {},
	"put":    {},
}

// UnmarshalJSON accepts full OpenAPI PathItem objects. In particular,
// `parameters` is an array at the path level, not an operation, so decoding a
// PathItem as map[string]OperationObject directly would reject valid specs.
func (p *PathItem) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var commonParams []ParameterObject
	if paramsRaw, ok := raw["parameters"]; ok {
		if err := json.Unmarshal(paramsRaw, &commonParams); err != nil {
			return fmt.Errorf("path-level parameters 파싱 실패: %w", err)
		}
	}

	item := make(PathItem)
	for key, value := range raw {
		method := strings.ToLower(key)
		if _, ok := supportedPathItemMethods[method]; !ok {
			continue
		}
		var op OperationObject
		if err := json.Unmarshal(value, &op); err != nil {
			return fmt.Errorf("%s operation 파싱 실패: %w", key, err)
		}
		op.Parameters = mergePathItemParameters(commonParams, op.Parameters)
		item[method] = op
	}

	*p = item
	return nil
}

func mergePathItemParameters(common, operation []ParameterObject) []ParameterObject {
	if len(common) == 0 {
		return operation
	}
	if len(operation) == 0 {
		return append([]ParameterObject(nil), common...)
	}

	overridden := make(map[string]struct{}, len(operation))
	for _, param := range operation {
		overridden[param.In+"\x00"+param.Name] = struct{}{}
	}

	merged := make([]ParameterObject, 0, len(common)+len(operation))
	for _, param := range common {
		if _, ok := overridden[param.In+"\x00"+param.Name]; ok {
			continue
		}
		merged = append(merged, param)
	}
	merged = append(merged, operation...)
	return merged
}

// OperationObject is one HTTP method on a path.
type OperationObject struct {
	OperationID string             `json:"operationId,omitempty"`
	Summary     string             `json:"summary,omitempty"`
	Description string             `json:"description,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	Parameters  []ParameterObject  `json:"parameters,omitempty"`
	RequestBody *RequestBodyObject `json:"requestBody,omitempty"`
	Responses   map[string]any     `json:"responses,omitempty"`
}

// ParameterObject is one path/query/header parameter.
type ParameterObject struct {
	Name        string        `json:"name"`
	In          string        `json:"in"` // "path" | "query" | "header"
	Required    bool          `json:"required,omitempty"`
	Description string        `json:"description,omitempty"`
	Schema      *SchemaObject `json:"schema,omitempty"`
}

// RequestBodyObject covers requestBody.
type RequestBodyObject struct {
	Required bool                       `json:"required,omitempty"`
	Content  map[string]MediaTypeObject `json:"content,omitempty"`
}

// MediaTypeObject covers content[mime].
type MediaTypeObject struct {
	Schema *SchemaObject `json:"schema,omitempty"`
}

// SchemaObject is a JSON Schema-ish blob. The CRM CLI only reads `type` and
// `enum`; other fields are accepted but unused.
type SchemaObject struct {
	Type        string                   `json:"type,omitempty"`
	Properties  map[string]*SchemaObject `json:"properties,omitempty"`
	Items       *SchemaObject            `json:"items,omitempty"`
	Enum        []string                 `json:"enum,omitempty"`
	Description string                   `json:"description,omitempty"`
	Ref         string                   `json:"$ref,omitempty"`
	Required    []string                 `json:"required,omitempty"`
}

// MappedCommand is one CLI subcommand derived from a (path, method) pair.
type MappedCommand struct {
	Resource     string
	Action       string
	Method       string // uppercase
	Path         string // original path including prefix
	Summary      string
	Tags         []string
	PathParams   []ParameterObject
	QueryParams  []ParameterObject
	HasBody      bool
	BodyRequired bool
}
