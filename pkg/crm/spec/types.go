// Package spec is the CRM OpenAPI loader, on-disk cache, and command mapper.
//
// The implementation mirrors @solapi/crm-cli (sdk/cli/src/spec/*.ts) so that
// solactl exposes the same dynamic `<resource> <action>` tree as the upstream
// node CLI. See docs/crm-cli-spec.md.
package spec

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
// Non-operation keys (e.g. "parameters", "summary") are tolerated by the
// mapper which checks each value for the OperationObject shape.
type PathItem map[string]OperationObject

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
