package tool

import (
	"fmt"
	"reflect"
	"strings"
)

// SchemaFromType builds a JSON Schema object from a Go type (typically a struct).
func SchemaFromType(t reflect.Type) map[string]any {
	if t == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	schema := schemaForType(t)
	if schema == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	if _, ok := schema["type"]; !ok {
		schema["type"] = "object"
	}
	removeTitleFields(schema)
	return schema
}

// ExtractInputSchema returns JSON Schema for T (PyV2 _extract_input_schema equivalent for structs).
func ExtractInputSchema[T any]() map[string]any {
	var zero T
	return SchemaFromType(reflect.TypeOf(zero))
}

func schemaForType(t reflect.Type) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		return structSchema(t)
	case reflect.Slice, reflect.Array:
		return map[string]any{
			"type":  "array",
			"items": schemaForType(t.Elem()),
		}
	case reflect.Map:
		if t.Key().Kind() == reflect.String {
			valueSchema := schemaForType(t.Elem())
			if valueSchema == nil {
				valueSchema = map[string]any{}
			}
			return map[string]any{
				"type":                 "object",
				"additionalProperties": valueSchema,
			}
		}
		return map[string]any{"type": "object"}
	default:
		if s := primitiveSchema(t); s != nil {
			return s
		}
		return map[string]any{"type": "string"}
	}
}

func structSchema(t reflect.Type) map[string]any {
	props := map[string]any{}
	var required []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name, omitEmpty := jsonFieldName(field)
		if name == "" || name == "-" {
			continue
		}
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		prop := schemaForType(fieldType)
		if prop == nil {
			prop = map[string]any{"type": "string"}
		}
		if desc := field.Tag.Get("desc"); desc != "" {
			prop["description"] = desc
		} else if desc := field.Tag.Get("jsonschema"); strings.HasPrefix(desc, "description=") {
			prop["description"] = strings.TrimPrefix(desc, "description=")
		}
		props[name] = prop
		if !omitEmpty && field.Type.Kind() != reflect.Pointer {
			required = append(required, name)
		}
	}
	out := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

func jsonFieldName(field reflect.StructField) (name string, omitEmpty bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "-", false
	}
	if tag == "" {
		return field.Name, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "" {
		name = field.Name
	}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty
}

func primitiveSchema(t reflect.Type) map[string]any {
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	default:
		return nil
	}
}

func removeTitleFields(schema map[string]any) {
	delete(schema, "title")
	for _, key := range []string{"properties", "items", "additionalProperties", "$defs"} {
		raw, ok := schema[key]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case map[string]any:
			if key == "properties" || key == "$defs" {
				for _, child := range v {
					if cm, ok := child.(map[string]any); ok {
						removeTitleFields(cm)
					}
				}
			} else {
				removeTitleFields(v)
			}
		}
	}
}

func decodeInput[T any](input map[string]any) (T, error) {
	var out T
	if input == nil {
		return out, fmt.Errorf("tool: nil input")
	}
	v := reflect.ValueOf(&out).Elem()
	t := v.Type()
	if t.Kind() != reflect.Struct {
		return out, fmt.Errorf("tool: auto decode requires struct input type")
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name, _ := jsonFieldName(field)
		if name == "" || name == "-" {
			continue
		}
		raw, ok := input[name]
		if !ok {
			continue
		}
		if err := setFieldValue(v.Field(i), raw); err != nil {
			return out, fmt.Errorf("tool: decode field %s: %w", name, err)
		}
	}
	return out, nil
}

func setFieldValue(field reflect.Value, raw any) error {
	if !field.CanSet() {
		return nil
	}
	val := reflect.ValueOf(raw)
	ft := field.Type()
	if val.IsValid() && val.Type().AssignableTo(ft) {
		field.Set(val)
		return nil
	}
	if val.IsValid() && val.Type().ConvertibleTo(ft) {
		field.Set(val.Convert(ft))
		return nil
	}
	switch ft.Kind() {
	case reflect.String:
		if s, ok := raw.(string); ok {
			field.SetString(s)
			return nil
		}
	case reflect.Bool:
		if b, ok := raw.(bool); ok {
			field.SetBool(b)
			return nil
		}
	case reflect.Int, reflect.Int64:
		switch n := raw.(type) {
		case float64:
			field.SetInt(int64(n))
			return nil
		case int:
			field.SetInt(int64(n))
			return nil
		case int64:
			field.SetInt(n)
			return nil
		}
	case reflect.Float64, reflect.Float32:
		switch n := raw.(type) {
		case float64:
			field.SetFloat(n)
			return nil
		case int:
			field.SetFloat(float64(n))
			return nil
		}
	}
	return fmt.Errorf("unsupported value %T for %s", raw, ft)
}
