package eval

import (
	"os"
	"strings"
)

type variables struct {
	values map[string]string
	// Whether a variable is exported or readonly are independent of whether it
	// is set, so we keep those attributes in separate maps.
	exported set[string]
	readonly set[string]
}

func initVariablesFromEnv(entries []string) variables {
	v := variables{
		values:   make(map[string]string, len(entries)),
		exported: make(set[string], len(entries)),
		readonly: make(set[string]),
	}
	for _, entry := range entries {
		// Note: Treat "foo" like "foo=" if such entries ever occur.
		name, value, _ := strings.Cut(entry, "=")
		v.values[name] = value
		v.exported.add(name)
	}
	wd, err := os.Getwd()
	if err == nil {
		v.values["PWD"] = wd
		v.exported.add("PWD")
	}
	return v
}

func (v variables) serializeEnvEntries() []string {
	entries := make([]string, 0, len(v.exported))
	for name := range v.exported {
		if value, ok := v.values[name]; ok {
			// Only variables that are both set and exported are exported to the
			// environment of child processes.
			entries = append(entries, name+"="+value)
		}
	}
	return entries
}

func (v variables) clone() variables {
	return variables{cloneMap(v.values), cloneMap(v.exported), cloneMap(v.readonly)}
}

// These are methods on [*frame] rather than [variables] because the behavior
// of setting variable depends on the [allexport] option.

func (fm *frame) GetVar(name string) string {
	return fm.variables.values[name]
}

func (fm *frame) SetVar(name, value string) bool {
	if fm.variables.readonly.has(name) {
		return false
	}
	if fm.options.has(allexport) {
		fm.variables.exported.add(name)
	}
	fm.variables.values[name] = value
	return true
}
