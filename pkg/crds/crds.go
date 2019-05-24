package crds

import (
	"bytes"
	"fmt"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"io"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// CRDs is a holder for a set of Tekton CRDs that define the pipeline to run
type CRDs struct {
	Pipelines map[string]pipeline.Pipeline
	Tasks     map[string]pipeline.Task
}

// NewCRDs create an empty CRDs
func NewCRDs() CRDs {
	return CRDs{
		Pipelines: map[string]pipeline.Pipeline{},
		Tasks:     map[string]pipeline.Task{},
	}
}

// ParseCRDs convert yaml files in folder into a set of CRDs
func ParseCRDs(r io.Reader) (*CRDs, error) {
	crds := NewCRDs()

	var buf bytes.Buffer
	tee := io.TeeReader(r, &buf)
	kindDecoder := yaml.NewYAMLToJSONDecoder(tee)
	fulldecoder := yaml.NewYAMLToJSONDecoder(&buf)

	for {
		t := meta.TypeMeta{}
		err := kindDecoder.Decode(&t)
		if err != nil {
			if err == io.EOF {
				// nothing mode to read
				break
			}
			return nil, fmt.Errorf("failed to parse yaml document. %v", err)
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
			return nil, fmt.Errorf("unsupported CRD 🧐 " + t.Kind)
		}
	}
	return &crds, nil
}
