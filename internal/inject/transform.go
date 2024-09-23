// Package inject ...
package inject

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/util/yaml"
	yamlconvert "sigs.k8s.io/yaml"
)

const readerSize = 4096

// Inputs Input slice
type Inputs []Input

// Input ...
type Input struct {
	ArgNumber  int
	Identifier string
	Reader     io.Reader
}

// Transformer ...
type Transformer struct {
	PatchGenerator PatchGenerator
	Inputs         Inputs
	Output         io.Writer
}

// ArgumentsToInputs ...
func ArgumentsToInputs(args []string) (Inputs, error) {
	inputs := Inputs{}
	for i, arg := range args {
		if arg == "-" {
			input := Input{
				ArgNumber:  i,
				Identifier: arg,
				Reader:     os.Stdin,
			}
			inputs = append(inputs, input)
			continue
		}

		file, err := os.Open(arg)
		if err != nil {
			return nil, fmt.Errorf("failed to open input(%d): %s, error: %w", i, arg, err)
		}

		input := Input{
			ArgNumber:  i,
			Identifier: arg,
			Reader:     file,
		}
		inputs = append(inputs, input)
	}

	return inputs, nil
}

// Transform ...
func (t *Transformer) Transform() error {
	first := true
	for _, v := range t.Inputs {
		if !first {
			_, err := t.Output.Write([]byte("---\n"))
			if err != nil {
				return fmt.Errorf("failed to write to standard output stream, error: %v", err)
			}
		}

		first = false
		err := t.transformInput(v)

		if err != nil {
			return fmt.Errorf("transformation failed for input: %s(%d), error: %w", v.Identifier, v.ArgNumber, err)
		}
	}

	return nil
}

func (t *Transformer) transformInput(input Input) error {
	reader := yaml.NewYAMLReader(bufio.NewReaderSize(input.Reader, readerSize))

	first := true
	for {
		bytes, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		obj, err := parseTypeMetaSkeleton(bytes)
		if err != nil {
			return err
		}

		if obj == nil {
			if !first {
				_, err = t.Output.Write([]byte("---\n"))
				if err != nil {
					return fmt.Errorf("failed to write to standard output stream, error: %v", err)
				}
			}

			_, err = t.Output.Write(bytes)
			if err != nil {
				return fmt.Errorf("failed to write to standard output stream, error: %v", err)
			}

			first = false
			fmt.Fprintf(os.Stderr, "unknown TypeMeta in input: (%d)%s, writing to output as-is\n", input.ArgNumber, input.Identifier)
			continue
		}

		err = yaml.Unmarshal(bytes, obj)
		if err != nil {
			return err
		}

		patchObj, err := t.PatchGenerator.Generate(context.TODO(), obj, "")
		if err != nil {
			return fmt.Errorf("failed to generate patch for kind: %T, error: %w", obj, err)
		}

		patchJSON, err := json.Marshal(patchObj)
		if err != nil {
			return err
		}

		patch, err := jsonpatch.DecodePatch(patchJSON)
		if err != nil {
			return err
		}

		origJSON, err := yamlconvert.YAMLToJSON(bytes)
		if err != nil {
			return err
		}

		injectedJSON, err := patch.Apply(origJSON)
		if err != nil {
			return err
		}

		injectedYAML, err := yamlconvert.JSONToYAML(injectedJSON)
		if err != nil {
			return err
		}

		if !first {
			_, err = t.Output.Write([]byte("---\n"))
			if err != nil {
				return fmt.Errorf("failed to write to standard output stream, error: %v", err)
			}
		}

		_, err = t.Output.Write(injectedYAML)
		if err != nil {
			return fmt.Errorf("failed to write to standard output stream, error: %v", err)
		}

		first = false
	}

	return nil
}
