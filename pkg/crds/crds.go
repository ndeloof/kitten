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
	Pipelines         map[string]pipeline.Pipeline
	Tasks             map[string]pipeline.Task
	PipelineResources map[string]pipeline.PipelineResource
}

// NewCRDs create an empty CRDs
func NewCRDs() CRDs {
	return CRDs{
		Pipelines: map[string]pipeline.Pipeline{},
		Tasks:     map[string]pipeline.Task{},
	}
}

// GetPipeline get a pipeline by name, or default one is there's only one declared
func (c CRDs) GetPipeline(name string) (pipeline.Pipeline, error) {
	p, ok := c.Pipelines[name]
	if ok {
		return p, nil
	}
	if name == "" && len(c.Pipelines) == 1 {
		for _, p := range c.Pipelines {
			return p, nil
		}
	}
	return p, fmt.Errorf("No pipeline with name '%s'\n", name)
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
		case "PipelineResource":
			to := pipeline.PipelineResource{}
			fulldecoder.Decode(&to)
			crds.PipelineResources[to.Name] = to
		default:
			return nil, fmt.Errorf("unsupported CRD 🧐 " + t.Kind)
		}
	}
	return &crds, nil
}
