package main

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/goware/prefixer"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipeline/dag"

	"fmt"
	"github.com/docker/docker/client"
	"github.com/ndeloof/kitten/pkg/crds"
	"io"
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

	r, err := os.Open(file)
	check(err, "Can't open CRDs file %s 😖", file)
	defer r.Close()

	crds, err := crds.ParseCRDs(r)
	check(err, "Failed to parse CRDs 😖")

	pipe, err := crds.GetPipeline(pname)
	check(err, "No pipeline found with name '%s' 🤗", pname)
	spec := pipe.Spec

	for _, r := range spec.Resources {
		pr, ok := crds.PipelineResources[r.Name]
		checkOk(ok, "No resource with name '%s' 🤗", r.Name)
		switch pr.Spec.Type {
		case "git":
			// TODO checkout
			fmt.Println("init git resource ...")
		case "image":
			// TODO docker pull
			fmt.Println("init image resource ...")
		}
	}

	cli, err := client.NewClientWithOpts(client.FromEnv)
	check(err, "Failed to setup docker client 🐳")

	ctx := context.Background()

	graph, err := pipeline.BuildDAG(spec.Tasks)
	check(err, "Invalid pipeline definition 🤕")
	completed := []string{}

	for {
		tasks, err := dag.GetSchedulable(graph, completed...)
		check(err, "Hum... sorry, we did it wrong 🤨")

		if len(tasks) == 0 {
			// We are done
			os.Exit(0)
		}

		for _, pt := range tasks {
			task, ok := crds.Tasks[pt.TaskRef.Name]
			checkOk(ok, "Pipeline TaskRef has no matching Task %s 🤭 ", pt.TaskRef.Name)
			status, err := RunTask(ctx, pt, task, cli)
			completed = append(completed, pt.Name)
			check(err, "Failed to run task ☠️")
			if status != 0 {
				os.Exit(status)
			}
		}
	}
}

// RunTask execute a Task using local docker runtime
func RunTask(ctx context.Context, pt pipeline.PipelineTask, task pipeline.Task, cli client.APIClient) (int, error) {
	fmt.Printf("Running Pipeline Task %s\n", pt.Name)
	for _, s := range task.Spec.Steps {
		fmt.Printf("--- running Step %s\n", s.Name)
		cmd := strslice.StrSlice{}
		cmd = append(cmd, s.Command...)
		cmd = append(cmd, s.Args...)

		cli.ImagePull(ctx, s.Image, types.ImagePullOptions{})

		c, err := cli.ContainerCreate(ctx,
			&container.Config{
				Image:        s.Image,
				Cmd:          cmd,
				AttachStdout: true,
				AttachStderr: true,
				Tty:          true,
				OpenStdin:    true, // so we wait for completion
			},
			&container.HostConfig{
				AutoRemove: true,
			},
			nil,
			"",
		)
		if err != nil {
			return 0, fmt.Errorf("can't create container. %v", err)
		}

		response, err := cli.ContainerAttach(ctx, c.ID, types.ContainerAttachOptions{
			Stream: true,
			Stdout: true,
			Stderr: true,
		})
		if err != nil {
			return 0, fmt.Errorf("can't attach container. %v ", err)
		}
		defer response.Close()

		statusChan, errChan := cli.ContainerWait(ctx, c.ID, container.WaitConditionNotRunning)

		prefix := fmt.Sprintf("[%s:%s] ", task.Name, s.Name)
		reader := prefixer.New(response.Reader, prefix)
		copyChan := make(chan error, 1)
		go func() {
			_, err := io.Copy(os.Stdout, reader)
			if err != nil {
				copyChan <- err
			}
		}()

		err = cli.ContainerStart(ctx, c.ID, types.ContainerStartOptions{})
		if err != nil {
			return 0, fmt.Errorf("can't start container. %v", err)
			// TODO cancel ioCopy func
		}

		select {
		case err := <-errChan:
			panic("Something wrong happened. " + err.Error())
		case status := <-statusChan:
			if status.StatusCode != 0 {
				return int(status.StatusCode), nil
			}
		case err := <-copyChan:
			panic("IO Copy went wrong. " + err.Error())
		}
	}
	return 0, nil
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
