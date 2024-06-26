package core

import (
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"text/template"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/encoding/yaml"
)

//go:embed schema.cue
var schemaFile string

func LoadTemplates(ctx *cue.Context) (cue.Value, error) {
	instances := load.Instances([]string{"."}, nil)
	if len(instances) == 0 {
		return cue.Value{}, fmt.Errorf("no CUE instance found in current directory")
	}

	instance := ctx.BuildInstance(instances[0])
	if err := instance.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("failed to build CUE instance: %v", err)
	}

	val := instance.Value()
	schema := ctx.CompileString(schemaFile)
	if err := schema.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("failed to compile schema: %v", err)
	}

	schemaVal := schema.LookupPath(cue.ParsePath("#Data"))
	if err := schemaVal.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("failed to lookup #Data in schema: %v", err)
	}

	val = val.Unify(schemaVal)
	if err := val.Validate(); err != nil {
		return cue.Value{}, fmt.Errorf("validation failed: %v", err)
	}

	return val, nil
}

func TraverseFields(val cue.Value, out io.Writer) error {
	iter, err := val.Fields()
	if err != nil {
		return fmt.Errorf("failed to iterate over CUE fields: %w", err)
	}

	for iter.Next() {
		key := iter.Label()
		value := iter.Value()
		fmt.Fprintf(out, "Key: %s\n", key)

		templatesIter, err := value.LookupPath(cue.ParsePath("templates")).List()
		if err != nil {
			return fmt.Errorf("failed to get templates list: %w", err)
		}

		for templatesIter.Next() {
			template := templatesIter.Value()
			templateStr, err := template.LookupPath(cue.ParsePath("template")).String()
			if err != nil {
				return fmt.Errorf("failed to get template string: %w", err)
			}
			fmt.Fprintf(out, "  Template: %s\n", templateStr)

			pathStr, err := template.LookupPath(cue.ParsePath("path")).String()
			if err != nil {
				return fmt.Errorf("failed to get path string: %w", err)
			}
			fmt.Fprintf(out, "  Path: %s\n", pathStr)
		}
		fmt.Fprintln(out)
	}

	return nil
}

const yamlTemplate = `# Do not edit, this is autogenerated from cue

{{ .Content }}`

func WriteYAML(val cue.Value, filename string) error {
	yamlBytes, err := yaml.Encode(val)
	if err != nil {
		return fmt.Errorf("failed to encode to YAML: %w", err)
	}

	tmpl, err := template.New("yaml").Parse(yamlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse YAML template: %w", err)
	}

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create YAML file: %w", err)
	}
	defer f.Close()

	data := map[string]string{
		"Content": string(yamlBytes),
	}

	err = tmpl.Execute(f, data)
	if err != nil {
		return fmt.Errorf("failed to write YAML file: %w", err)
	}

	return nil
}

func Run() {
	ctx := cuecontext.New()
	val, err := LoadTemplates(ctx)
	if err != nil {
		log.Fatal(err)
	}

	err = TraverseFields(val, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}

	err = WriteYAML(val, "templates.yaml")
	if err != nil {
		log.Fatal(err)
	}
}
