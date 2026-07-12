package spec

// [>] 🤖🤖

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	sj "github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/yaml.v3"
)

var printer = message.NewPrinter(language.English)

var compiledSchema = sync.OnceValues(func() (*sj.Schema, error) {
	b, err := json.Marshal(Schema())
	if err != nil {
		return nil, err
	}
	doc, err := sj.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	c := sj.NewCompiler()
	if err := c.AddResource("che.schema.json", doc); err != nil {
		return nil, err
	}
	return c.Compile("che.schema.json")
})

func CompiledSchema() (*sj.Schema, error) { return compiledSchema() }

func YAMLInstance(b []byte) (any, error) {
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	j, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return sj.UnmarshalJSON(bytes.NewReader(j))
}

func ValidateSchema(b []byte) []string {
	sch, err := compiledSchema()
	if err != nil {
		panic("spec: compile che.yml schema: " + err.Error())
	}
	inst, err := YAMLInstance(b)
	if err != nil {
		return nil
	}
	err = sch.Validate(inst)
	if err == nil {
		return nil
	}
	var ve *sj.ValidationError
	if !errors.As(err, &ve) {
		return []string{err.Error()}
	}
	var out []string
	seen := map[string]bool{}
	var walk func(e *sj.ValidationError)
	walk = func(e *sj.ValidationError) {
		if len(e.Causes) == 0 {
			s := instPath(e.InstanceLocation) + ": " + e.ErrorKind.LocalizedString(printer)
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	return out
}

func instPath(loc []string) string {
	if len(loc) == 0 {
		return "/"
	}
	return "/" + strings.Join(loc, "/")
}

// [<] 🤖🤖
