package crds

import (
	"fmt"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"io"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
	"path/filepath"
	"strings"
)

// CRDs is a holder for a set of Tekton CRDs that define the pipeline to run
type CRDs struct {
	Pipelines map[string]pipeline.Pipeline
	Tasks     map[string]pipeline.Task
}

// ParseCRDs convert yaml files in folder into a set of CRDs
func ParseCRDs(path string) (*CRDs, error) {

	crds := CRDs{
		Pipelines: map[string]pipeline.Pipeline{},
		Tasks:     map[string]pipeline.Task{},
	}
	var files []string
	err := filepath.Walk(path, func(f string, info os.FileInfo, err error) error {
		if strings.HasSuffix(f, ".yaml") {
			files = append(files, f)
		}
		return nil
	})
	if err != nil {
		return &crds, err
	}

	for _, file := range files {
		err := parseYamlIntoCRDs(&crds, file)
		if err != nil {
			return &crds, err
		}
	}
	return &crds, nil
}

func parseYamlIntoCRDs(crds *CRDs, f string) error {
	r, err := os.Open(f)
	defer r.Close()
	if err != nil {
		return fmt.Errorf("can't open file. %v", err)
	}
	decoder := yaml.NewYAMLToJSONDecoder(r)

	r2, err := os.Open(f)
	defer r2.Close()
	if err != nil {
		return fmt.Errorf("can't open file. %v", err)
	}
	fulldecoder := yaml.NewYAMLToJSONDecoder(r2)

	for {
		t := meta.TypeMeta{}
		err = decoder.Decode(&t)
		if err != nil {
			if err == io.EOF {
				// nothing mode to read
				break
			}
			return fmt.Errorf("failed to parse yaml document. %v", err)
		}
		switch t.Kind {
		case "Task":
			to := pipeline.Task{}
			fulldecoder.Decode(&to)
			crds.Tasks[to.Name] = to
		case "Pipeline":
			to := pipeline.Pipeline{}
			fulldecoder.Decode(&to)
			crds.Pipelines[to.Name] = to
		default:
			return fmt.Errorf("unsupported CRD 🧐 " + t.Kind)
		}
	}
	return nil
}
