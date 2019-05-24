package main

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/goware/prefixer"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipeline/dag"

	"flag"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/ndeloof/kitten/pkg/crds"
	"io"
	"os"
)

func main() {

	var file string
	var pname string
	flag.StringVar(&file, "file", "pipeline.yaml", "Path to Tekton pipeline CRDs")
	flag.StringVar(&pname, "pipeline", "", "Pipeline to run (if more than one defined)")
	flag.Parse()

	r, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't open file %s", file)
		os.Exit(1)
	}
	defer r.Close()

	crds, err := crds.ParseCRDs(r)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	pipe := crds.Pipelines[pname]
	if pname == "" && len(crds.Pipelines) == 1 {
		for _, p := range crds.Pipelines {
			pipe = p
		}
	}
	spec := pipe.Spec.Tasks

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic("no docker 🐳 " + err.Error())
	}

	ctx := context.Background()

	graph, err := pipeline.BuildDAG(spec)
	if err != nil {
		panic("invalid pipeline 🤕 " + err.Error())
	}
	completed := []string{}

	for {
		tasks, err := dag.GetSchedulable(graph, completed...)
		if err != nil {
			panic("we did it wrong 🤨 " + err.Error())
		}

		if len(tasks) == 0 {
			// We are done
			os.Exit(0)
		}

		for _, pt := range tasks {
			task, ok := crds.Tasks[pt.TaskRef.Name]
			if !ok {
				panic("Pipeline TaskRef has no matching Task 🤭 " + pt.TaskRef.Name)
			}
			status, err := RunTask(ctx, pt, task, cli)
			completed = append(completed, pt.Name)
			if err != nil {
				panic("Failed to run ☠️ " + err.Error())
			}
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
