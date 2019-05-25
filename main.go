package main

import (
	"fmt"
	"github.com/ndeloof/kitten/pkg/crds"
	"github.com/ndeloof/kitten/pkg/kitten"
	"os"
)

func main() {
	args := os.Args
	if len(args) == 0 {
		fmt.Println("Usage: kitten <crds> [<name>]")
		fmt.Println("args:")
		fmt.Println(" - crds: a (multi-documents) yaml file containing CRDs to define your Tekton pipeline")
		fmt.Println(" - name: name of the pipeline to run. Optional if CRDs only define a single pipeline")
	}

	file := args[1]
	var pname string
	if len(args) > 2 {
		pname = args[2]
	}

	var r *os.File
	var err error
	if file == "-" {
		r = os.Stdin
	} else {
		r, err = os.Open(file)
		check(err, "Can't open CRDs file %s 😖", file)
	}
	defer r.Close()

	crds, err := crds.ParseCRDs(r)
	check(err, "Failed to parse CRDs 😖")

	run, err := crds.GetPipelineRun(pname)
	check(err, "No pipelineRun found with name '%s' 🤗", pname)

	status, err := kitten.ExecutePipelineRun(crds, run)
	check(err, "Failed to execute PipelineRun. If you think this is a bug, please report an issue.")
	os.Exit(status)
}

func check(err error, message string, s ...interface{}) {
	if err != nil {
		fmt.Fprintf(os.Stderr, message, s...)
		fmt.Println()
		fmt.Fprintf(os.Stderr, err.Error())
		fmt.Println()
		os.Exit(1)
	}
}

func checkOk(ok bool, message string, s ...interface{}) {
	if !ok {
		fmt.Fprintf(os.Stderr, message, s...)
		fmt.Println()
		os.Exit(1)
	}
}
