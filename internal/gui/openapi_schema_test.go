package gui

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/settings"
)

// The schemas of openapi.json are written by hand; this holds them to the
// Go types magpie writes and reads. A schema that lists properties lists
// every field of its type, each with the type it has, and requires exactly
// those always sent (no omitempty). A schema with no properties ({"type":
// "object"}) leaves its object unsaid and is not looked into.
func TestOpenAPISchemasMatchTypes(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(openAPI, &doc); err != nil {
		t.Fatal(err)
	}
	c := schemaCheck{doc: doc, seen: map[string]bool{}}
	for name, typ := range map[string]reflect.Type{
		"State":            reflect.TypeFor[stateJSON](),
		"SettingsState":    reflect.TypeFor[settingsJSON](),
		"Settings":         reflect.TypeFor[settings.Settings](),
		"ProviderState":    reflect.TypeFor[providersJSON](),
		"SignInStatus":     reflect.TypeFor[provider.SignInState](),
		"AgentFieldChange": reflect.TypeFor[agentFieldChange](),
	} {
		c.check(name, typ, map[string]any{"$ref": "#/components/schemas/" + name})
	}
	sort.Strings(c.errs)
	for _, e := range c.errs {
		t.Error(e)
	}
}

// agentFieldChange is the body of POST /api/set, which the handler reads
// into an anonymous struct.
type agentFieldChange struct {
	Agent string `json:"agent"`
	Field string `json:"field"`
	Value string `json:"value"`
}

type schemaCheck struct {
	doc  map[string]any
	seen map[string]bool
	errs []string
}

func (c *schemaCheck) fail(at, format string, args ...any) {
	c.errs = append(c.errs, at+": "+fmt.Sprintf(format, args...))
}

// resolve follows $ref and merges allOf into one schema.
func (c *schemaCheck) resolve(s map[string]any) map[string]any {
	if ref, ok := s["$ref"].(string); ok {
		at := any(c.doc)
		for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			at = at.(map[string]any)[part]
		}
		return c.resolve(at.(map[string]any))
	}
	all, ok := s["allOf"].([]any)
	if !ok {
		return s
	}
	out := map[string]any{"type": "object"}
	props := map[string]any{}
	var required []any
	for _, part := range all {
		p := c.resolve(part.(map[string]any))
		for k, v := range p {
			switch k {
			case "properties":
				for name, prop := range v.(map[string]any) {
					props[name] = prop
				}
			case "required":
				required = append(required, v.([]any)...)
			default:
				out[k] = v
			}
		}
	}
	if len(props) > 0 {
		out["properties"] = props
	}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

func schemaTypes(s map[string]any) []string {
	switch v := s["type"].(type) {
	case string:
		return []string{v}
	case []any:
		var out []string
		for _, x := range v {
			out = append(out, x.(string))
		}
		return out
	}
	return nil
}

var timeType = reflect.TypeFor[time.Time]()

// jsonType is the JSON type values of t are written as, "" for any.
func jsonType(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == timeType {
		return "string"
	}
	if t.Implements(reflect.TypeFor[json.Marshaler]()) || reflect.PointerTo(t).Implements(reflect.TypeFor[json.Marshaler]()) {
		return ""
	}
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return "string"
		}
		return "array"
	case reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	}
	return ""
}

type jsonField struct {
	typ       reflect.Type
	omitEmpty bool
}

// jsonFields are the fields encoding/json writes for struct t, embedded
// structs' promoted.
func jsonFields(t reflect.Type) map[string]jsonField {
	out := map[string]jsonField{}
	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if f.Anonymous && name == "" {
			et := f.Type
			if et.Kind() == reflect.Pointer {
				et = et.Elem()
			}
			if et.Kind() == reflect.Struct {
				for k, v := range jsonFields(et) {
					if _, own := out[k]; !own {
						out[k] = v
					}
				}
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		out[name] = jsonField{typ: f.Type, omitEmpty: slices.Contains(strings.Split(opts, ","), "omitempty") || slices.Contains(strings.Split(opts, ","), "omitzero")}
	}
	return out
}

func (c *schemaCheck) check(at string, t reflect.Type, raw map[string]any) {
	if ref, ok := raw["$ref"].(string); ok {
		key := ref + " " + t.String()
		if c.seen[key] {
			return
		}
		c.seen[key] = true
	}
	if _, ok := raw["oneOf"]; ok {
		return
	}
	s := c.resolve(raw)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	want := jsonType(t)
	types := schemaTypes(s)
	if want == "" || len(types) == 0 {
		return
	}
	if !slices.Contains(types, want) && !(want == "integer" && slices.Contains(types, "number")) {
		c.fail(at, "documented as %v, but magpie's %s is a JSON %s", types, t, want)
		return
	}
	switch want {
	case "array":
		if items, ok := s["items"].(map[string]any); ok {
			c.check(at+"[]", t.Elem(), items)
		}
	case "object":
		if t.Kind() == reflect.Map {
			if extra, ok := s["additionalProperties"].(map[string]any); ok {
				c.check(at+"{}", t.Elem(), extra)
			}
			return
		}
		props, ok := s["properties"].(map[string]any)
		if !ok {
			return
		}
		fields := jsonFields(t)
		required := map[string]bool{}
		if r, ok := s["required"].([]any); ok {
			for _, x := range r {
				required[x.(string)] = true
			}
		}
		for name := range props {
			if _, ok := fields[name]; !ok {
				c.fail(at, "documents %q, which %s does not have", name, t)
			}
		}
		for name := range required {
			if _, ok := props[name]; !ok {
				c.fail(at, "requires %q, which it does not document", name)
			}
		}
		for name, f := range fields {
			prop, ok := props[name].(map[string]any)
			if !ok {
				c.fail(at, "does not document %q, which %s sends", name, t)
				continue
			}
			switch {
			case f.omitEmpty && required[name]:
				c.fail(at, "requires %q, which %s leaves out when empty", name, t)
			case !f.omitEmpty && !required[name]:
				c.fail(at, "does not require %q, which %s always sends", name, t)
			}
			c.check(at+"."+name, f.typ, prop)
		}
	}
}
