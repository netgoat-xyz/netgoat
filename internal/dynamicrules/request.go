package dynamicrules

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dop251/goja"
)

type namedValues struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

// normalizedRequest is deliberately map-free. It gives the VM stable property
// insertion order regardless of the random iteration order of Go maps.
type normalizedRequest struct {
	Method   string        `json:"method"`
	Scheme   string        `json:"scheme"`
	Host     string        `json:"host"`
	Path     string        `json:"path"`
	RawQuery string        `json:"rawQuery"`
	Query    []namedValues `json:"query"`
	Headers  []namedValues `json:"headers"`
	Body     string        `json:"body"`
	ClientIP string        `json:"clientIP"`
}

func normalizeRequest(request Request, maxBytes int) (normalizedRequest, error) {
	if maxBytes <= 0 {
		return normalizedRequest{}, fmt.Errorf("%w: input limit must be positive", ErrLimitExceeded)
	}
	maxProperties := maxRequestProperties(maxBytes)
	if len(request.Headers)+len(request.Query) > maxProperties {
		return normalizedRequest{}, limitError("request property count", len(request.Headers)+len(request.Query), maxProperties)
	}

	headers, err := canonicalValues(request.Headers, true, maxProperties)
	if err != nil {
		return normalizedRequest{}, fmt.Errorf("headers: %w", err)
	}
	query, err := canonicalValues(request.Query, false, maxProperties)
	if err != nil {
		return normalizedRequest{}, fmt.Errorf("query: %w", err)
	}

	path := request.Path
	if path == "" {
		path = "/"
	}
	normalized := normalizedRequest{
		Method:   strings.TrimSpace(request.Method),
		Scheme:   strings.ToLower(strings.TrimSpace(request.Scheme)),
		Host:     strings.ToLower(strings.TrimSpace(request.Host)),
		Path:     path,
		RawQuery: request.RawQuery,
		Query:    query,
		Headers:  headers,
		Body:     request.Body,
		ClientIP: strings.TrimSpace(request.ClientIP),
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"method", normalized.Method},
		{"scheme", normalized.Scheme},
		{"host", normalized.Host},
		{"path", normalized.Path},
		{"raw query", normalized.RawQuery},
		{"body", normalized.Body},
		{"client IP", normalized.ClientIP},
	} {
		if !utf8.ValidString(field.value) {
			return normalizedRequest{}, fmt.Errorf("request %s is not valid UTF-8", field.name)
		}
	}

	// JSON is only used to account for the full, canonical payload. The VM gets
	// native JavaScript objects built below, never a Go map wrapped by Goja.
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return normalizedRequest{}, fmt.Errorf("encode canonical request: %w", err)
	}
	if len(encoded) > maxBytes {
		return normalizedRequest{}, limitError("request input", len(encoded), maxBytes)
	}
	return normalized, nil
}

func canonicalValues(values map[string][]string, foldKeys bool, maxProperties int) ([]namedValues, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) > maxProperties {
		return nil, limitError("request property count", len(values), maxProperties)
	}

	rawKeys := make([]string, 0, len(values))
	for key := range values {
		rawKeys = append(rawKeys, key)
	}
	sort.Strings(rawKeys)

	combined := make(map[string][]string, len(values))
	valueCount := 0
	for _, rawKey := range rawKeys {
		if !utf8.ValidString(rawKey) {
			return nil, fmt.Errorf("key is not valid UTF-8")
		}
		key := rawKey
		if foldKeys {
			key = strings.ToLower(strings.TrimSpace(key))
		}
		if key == "" {
			return nil, fmt.Errorf("empty key")
		}
		for _, value := range values[rawKey] {
			if !utf8.ValidString(value) {
				return nil, fmt.Errorf("value for %q is not valid UTF-8", rawKey)
			}
			valueCount++
			if valueCount > maxProperties {
				return nil, limitError("request value count", valueCount, maxProperties)
			}
			combined[key] = append(combined[key], value)
		}
	}

	keys := make([]string, 0, len(combined))
	for key := range combined {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]namedValues, 0, len(keys))
	for _, key := range keys {
		result = append(result, namedValues{Key: key, Values: combined[key]})
	}
	return result, nil
}

func maxRequestProperties(maxBytes int) int {
	// A property needs at least a key, punctuation, and a JSON value. This
	// separate cap prevents a giant sparse Go map from forcing unbounded sorting
	// before the byte accounting below can reject it.
	max := maxBytes / 2
	if max < 1 {
		return 1
	}
	if max > 4096 {
		return 4096
	}
	return max
}

func cloneStringValues(values map[string][]string) map[string][]string {
	if len(values) == 0 {
		return nil
	}
	copyValues := make(map[string][]string, len(values))
	for key, items := range values {
		copyValues[key] = append([]string(nil), items...)
	}
	return copyValues
}

func (request normalizedRequest) value(rt *goja.Runtime) (*goja.Object, error) {
	query, err := namedValuesObject(rt, request.Query)
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	headers, err := namedValuesObject(rt, request.Headers)
	if err != nil {
		return nil, fmt.Errorf("build headers: %w", err)
	}

	object, err := newDataObject(rt)
	if err != nil {
		return nil, err
	}
	for _, field := range []struct {
		name  string
		value goja.Value
	}{
		{"method", rt.ToValue(request.Method)},
		{"scheme", rt.ToValue(request.Scheme)},
		{"host", rt.ToValue(request.Host)},
		{"path", rt.ToValue(request.Path)},
		{"rawQuery", rt.ToValue(request.RawQuery)},
		{"query", query},
		{"headers", headers},
		{"body", rt.ToValue(request.Body)},
		{"clientIP", rt.ToValue(request.ClientIP)},
	} {
		if err := object.DefineDataProperty(field.name, field.value, goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE); err != nil {
			return nil, fmt.Errorf("define request %s: %w", field.name, err)
		}
	}
	if err := freezeObject(rt, object); err != nil {
		return nil, fmt.Errorf("freeze request: %w", err)
	}
	return object, nil
}

func namedValuesObject(rt *goja.Runtime, values []namedValues) (*goja.Object, error) {
	object, err := newDataObject(rt)
	if err != nil {
		return nil, err
	}
	for _, entry := range values {
		array := rt.NewArray()
		for index, value := range entry.Values {
			if err := array.Set(strconv.Itoa(index), value); err != nil {
				return nil, fmt.Errorf("set %q value: %w", entry.Key, err)
			}
		}
		if err := freezeObject(rt, array); err != nil {
			return nil, fmt.Errorf("freeze %q values: %w", entry.Key, err)
		}
		if err := object.DefineDataProperty(entry.Key, array, goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE); err != nil {
			return nil, fmt.Errorf("define %q: %w", entry.Key, err)
		}
	}
	if err := freezeObject(rt, object); err != nil {
		return nil, err
	}
	return object, nil
}

func newDataObject(rt *goja.Runtime) (*goja.Object, error) {
	object := rt.NewObject()
	if err := object.SetPrototype(nil); err != nil {
		return nil, fmt.Errorf("set null prototype: %w", err)
	}
	return object, nil
}

func freezeObject(rt *goja.Runtime, object *goja.Object) error {
	globalObject := rt.Get("Object")
	if globalObject == nil {
		return fmt.Errorf("Object is unavailable")
	}
	freezeValue := globalObject.ToObject(rt).Get("freeze")
	freeze, ok := goja.AssertFunction(freezeValue)
	if !ok {
		return fmt.Errorf("Object.freeze is unavailable")
	}
	_, err := freeze(globalObject, object)
	return err
}
