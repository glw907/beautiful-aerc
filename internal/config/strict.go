package config

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/glw907/poplar/internal/strdist"
)

// strictDecode decodes data into v, then rejects any TOML keys the
// destination struct does not know about. Unknown keys surface as
// *ConfigError with a Levenshtein-based Suggest pointing at the
// nearest sibling field.
func strictDecode(data []byte, v any) error {
	md, err := toml.NewDecoder(bytes.NewReader(data)).Decode(v)
	if err != nil {
		return err
	}
	rootType := reflect.TypeOf(v)
	for _, key := range md.Undecoded() {
		path := []string(key)
		if outOfScope(rootType, path) || dropsIntoMap(rootType, path) {
			continue
		}
		field := path[len(path)-1]
		dotted := strings.Join(path, ".")
		suggest := suggestSibling(rootType, path[:len(path)-1], field)
		ce := &ConfigError{Field: dotted, Message: fmt.Sprintf("unknown key %q", dotted)}
		if suggest != "" {
			ce.Suggest = fmt.Sprintf(`did you mean "%s"?`, suggest)
		}
		return ce
	}
	return nil
}

// outOfScope returns true when the top-level segment of path is not a
// known field of the destination struct. Configs share a single file
// across [[account]], [ui], and [cache] sections. Each Load* only
// knows its own; unknowns rooted at a sibling section belong to
// somebody else, not this caller.
func outOfScope(t reflect.Type, path []string) bool {
	if len(path) == 0 {
		return true
	}
	t = derefAndElem(t)
	if t.Kind() != reflect.Struct {
		return false
	}
	_, ok := findField(t, path[0])
	return !ok
}

// dropsIntoMap reports whether the given dotted path lands inside a
// map[string]... field. Map fields accept arbitrary keys; Undecoded
// entries inside them are not errors.
func dropsIntoMap(t reflect.Type, path []string) bool {
	for i, segment := range path {
		t = derefAndElem(t)
		if t.Kind() == reflect.Map {
			return true
		}
		if t.Kind() != reflect.Struct {
			return false
		}
		ft, ok := findField(t, segment)
		if !ok {
			// Last segment is the unknown key itself. Everything before
			// it should have resolved.
			return i < len(path)-1
		}
		t = ft.Type
	}
	return false
}

// suggestSibling returns the closest known field name (within
// Levenshtein distance 2) at the given struct path.
func suggestSibling(t reflect.Type, parentPath []string, field string) string {
	t = derefAndElem(t)
	for _, segment := range parentPath {
		if t.Kind() == reflect.Map {
			t = derefAndElem(t.Elem())
			continue
		}
		if t.Kind() != reflect.Struct {
			return ""
		}
		ft, ok := findField(t, segment)
		if !ok {
			return ""
		}
		t = derefAndElem(ft.Type)
	}
	if t.Kind() != reflect.Struct {
		return ""
	}
	best := ""
	bestDist := 3
	for i := range t.NumField() {
		name := tomlName(t.Field(i))
		if name == "" || name == "-" {
			continue
		}
		if d := strdist.Levenshtein(field, name, 2); d < bestDist {
			bestDist = d
			best = name
		}
	}
	return best
}

func findField(t reflect.Type, name string) (reflect.StructField, bool) {
	for i := range t.NumField() {
		if tomlName(t.Field(i)) == name {
			return t.Field(i), true
		}
	}
	return reflect.StructField{}, false
}

func tomlName(sf reflect.StructField) string {
	tag := sf.Tag.Get("toml")
	if tag == "" {
		return strings.ToLower(sf.Name)
	}
	if comma := strings.IndexByte(tag, ','); comma >= 0 {
		tag = tag[:comma]
	}
	return tag
}

func derefAndElem(t reflect.Type) reflect.Type {
	for {
		switch t.Kind() {
		case reflect.Ptr:
			t = t.Elem()
		case reflect.Slice, reflect.Array:
			t = t.Elem()
		default:
			return t
		}
	}
}
