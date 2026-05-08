package spec

import (
	"sort"
	"strconv"
	"strings"
)

// PathPrefix is the path prefix that mapper accepts. Paths outside this
// prefix are silently skipped (mirrors mapper.ts:31-34).
const PathPrefix = "/crm-core/v1/"

var actionByMethod = map[string]string{
	"get":    "get",
	"post":   "create",
	"patch":  "update",
	"put":    "replace",
	"delete": "delete",
}

// MapSpec extracts MappedCommand entries from an OpenAPI spec. Iteration
// order is deterministic (paths sorted) so duplicate-action suffixing is
// reproducible across runs and tests.
func MapSpec(spec *OpenApiSpec) []MappedCommand {
	if spec == nil || len(spec.Paths) == 0 {
		return nil
	}

	paths := make([]string, 0, len(spec.Paths))
	for p := range spec.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var commands []MappedCommand
	for _, p := range paths {
		if !strings.HasPrefix(p, PathPrefix) {
			continue
		}
		relative := strings.TrimPrefix(p, PathPrefix)
		segments := strings.Split(relative, "/")
		if len(segments) == 0 || segments[0] == "" {
			continue
		}
		resource := segments[0]

		methods := sortedMethods(spec.Paths[p])
		for _, method := range methods {
			op := spec.Paths[p][method]
			if !isOperation(op) {
				continue
			}
			action := deriveAction(method, segments, op)

			pathParams := make([]ParameterObject, 0, len(op.Parameters))
			queryParams := make([]ParameterObject, 0, len(op.Parameters))
			for _, param := range op.Parameters {
				switch param.In {
				case "path":
					pathParams = append(pathParams, param)
				case "query":
					queryParams = append(queryParams, param)
				}
			}

			commands = append(commands, MappedCommand{
				Resource:     resource,
				Action:       action,
				Method:       strings.ToUpper(method),
				Path:         p,
				Summary:      op.Summary,
				Tags:         op.Tags,
				PathParams:   pathParams,
				QueryParams:  queryParams,
				HasBody:      op.RequestBody != nil,
				BodyRequired: op.RequestBody != nil && op.RequestBody.Required,
			})
		}
	}

	dedupeActions(commands)
	return commands
}

// isOperation distinguishes a real OpenAPI operation entry from a zero-valued
// struct produced by JSON keys that are not HTTP methods (e.g. "parameters"
// at the PathItem level, or empty `{}` placeholders).
func isOperation(o OperationObject) bool {
	return o.OperationID != "" || o.Summary != "" || o.Description != "" ||
		len(o.Tags) > 0 || o.Parameters != nil || o.RequestBody != nil ||
		o.Responses != nil
}

func sortedMethods(item PathItem) []string {
	out := make([]string, 0, len(item))
	for m := range item {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// dedupeActions disambiguates same (resource, action) entries by appending
// a sub-path suffix, then a counter when the suffix is empty.
func dedupeActions(commands []MappedCommand) {
	seen := make(map[string]int)
	for i := range commands {
		cmd := &commands[i]
		key := cmd.Resource + ":" + cmd.Action
		count := seen[key]
		if count > 0 {
			suffix := extractActionSuffix(cmd.Path)
			if suffix != "" {
				cmd.Action = cmd.Action + "-" + suffix
			} else {
				cmd.Action = cmd.Action + "-" + strconv.Itoa(count+1)
			}
		}
		seen[key] = count + 1
	}
}

// extractActionSuffix joins the static (non-parameter) segments past the
// resource with '-'. Examples:
//
//	/crm-core/v1/records/trash                       -> "trash"
//	/crm-core/v1/records/bulk/restore                -> "bulk-restore"
//	/crm-core/v1/records/search/fulltext/{entityId}  -> "search-fulltext"
func extractActionSuffix(path string) string {
	relative := strings.TrimPrefix(path, PathPrefix)
	segments := strings.Split(relative, "/")
	if len(segments) <= 1 {
		return ""
	}
	staticParts := make([]string, 0, len(segments)-1)
	for _, s := range segments[1:] {
		if s == "" || strings.HasPrefix(s, "{") || strings.HasPrefix(s, ":") {
			continue
		}
		staticParts = append(staticParts, s)
	}
	return strings.Join(staticParts, "-")
}

// deriveAction maps (method, segments) to a CLI action name.
func deriveAction(method string, segments []string, op OperationObject) string {
	method = strings.ToLower(method)
	baseAction, ok := actionByMethod[method]
	if !ok {
		baseAction = method
	}

	subSegments := segments[1:]
	staticParts := make([]string, 0, len(subSegments))
	for _, s := range subSegments {
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, "{") || strings.HasPrefix(s, ":") {
			continue
		}
		staticParts = append(staticParts, s)
	}

	var lastSegment string
	if len(segments) > 0 {
		lastSegment = segments[len(segments)-1]
	}
	isLastParam := strings.HasPrefix(lastSegment, "{") || strings.HasPrefix(lastSegment, ":")

	switch method {
	case "get":
		if len(segments) == 1 {
			return "list"
		}
		if isLastParam && len(staticParts) == 0 {
			return "get"
		}
		suffix := strings.Join(staticParts, "-")
		if suffix == "" {
			return "get"
		}
		return "list-" + suffix
	case "post":
		if len(segments) == 1 {
			return "create"
		}
		if len(staticParts) > 0 {
			return strings.Join(staticParts, "-")
		}
		return "create"
	}

	// PATCH/PUT/DELETE: prefer operationId of form "verb_name".
	if op.OperationID != "" {
		parts := strings.Split(op.OperationID, "_")
		if len(parts) > 1 {
			return strings.Join(parts[1:], "-")
		}
	}
	return baseAction
}

// Resources returns the sorted, de-duplicated list of resource names.
func Resources(commands []MappedCommand) []string {
	seen := make(map[string]struct{}, len(commands))
	for _, c := range commands {
		seen[c.Resource] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// CommandsForResource filters commands by resource, preserving original order.
func CommandsForResource(commands []MappedCommand, resource string) []MappedCommand {
	out := make([]MappedCommand, 0)
	for _, c := range commands {
		if c.Resource == resource {
			out = append(out, c)
		}
	}
	return out
}
