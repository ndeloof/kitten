package crds

import (
	_ "reflect"
	"testing"
)

func TestParseCRDs(t *testing.T) {
	crds, err := ParseCRDs("testdata")
	if err != nil {
		t.Fatal("Failed to parse CRDs", err)
	}
	if len(crds.Pipelines) != 1 {
		t.Fatal("Unexpected number of parsed Pipeline CRDs")
	}
	if len(crds.Tasks) != 2 {
		t.Fatal("Unexpected number of parsed Task CRDs")
	}
	_, ok := crds.Pipelines["sample-pipeline"]
	if !ok {
		t.Fatal("Should have parsed sample-pipeline Pipeline")
	}
	if _, ok := crds.Tasks["sample"]; !ok {
		t.Fatal("Should have parsed sample Task")
	}
	// reflect.DeepEqual
}
