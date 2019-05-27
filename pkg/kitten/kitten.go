package kitten

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/goware/prefixer"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipeline/dag"
	k8s "k8s.io/api/core/v1"

	"encoding/json"
	"fmt"
	"github.com/docker/docker/client"
	message "github.com/docker/docker/pkg/jsonmessage"
	"github.com/ndeloof/kitten/pkg/crds"
	"io"
	"os"
)

// ExecutePipelineRun executes a PipelineRun using a docker runtime
func ExecutePipelineRun(crds *crds.CRDs, run pipeline.PipelineRun) (int, error) {
	pref := run.Spec.PipelineRef.Name
	pipe, ok := crds.Pipelines[pref]
	if !ok {
		return 0, fmt.Errorf("No Pipeline found for PipelineRef '%s' 🤗", pref)
	}
	spec := pipe.Spec

	// map PipelineRun resources into actual resources
	for _, r := range pipe.Spec.Resources {
		pr, ok := crds.PipelineResources[r.Name]
		if !ok {
			return 0, fmt.Errorf("No resource with name '%s' 🤗", r.Name)
		}
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
	if err != nil {
		return 0, fmt.Errorf("Failed to setup docker client 🐳\n %v", err)
	}

	ctx := context.Background()

	graph, err := pipeline.BuildDAG(spec.Tasks)
	if err != nil {
		return 0, fmt.Errorf("Invalid pipeline definition 🤕\n %v", err)
	}
	completed := []string{}

	for {
		tasks, err := dag.GetSchedulable(graph, completed...)
		if err != nil {
			return 0, fmt.Errorf("Hum... sorry, we did it wrong 🤨\n %v", err)
		}

		if len(tasks) == 0 {
			// We are done
			return 0, nil
		}

		for _, pt := range tasks {
			task, ok := crds.Tasks[pt.TaskRef.Name]
			if !ok {
				return 0, fmt.Errorf("Pipeline TaskRef has no matching Task %s 🤭 ", pt.TaskRef.Name)
			}
			status, err := RunTask(ctx, pt, task, cli)
			completed = append(completed, pt.Name)
			if err != nil {
				return 0, fmt.Errorf("Failed to run task ☠️\n %v", err)
			}
			if status != 0 {
				return status, nil
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

		pull := s.ImagePullPolicy == k8s.PullAlways
		if s.ImagePullPolicy != k8s.PullNever {
			_, _, err := cli.ImageInspectWithRaw(ctx, s.Image)
			if err != nil {
				if client.IsErrNotFound(err) {
					pull = true
				} else {
					return 0, fmt.Errorf("can't inspect image. %v", err)
				}
			}
		}

		if pull {
			fmt.Printf("Pulling Docker image %s\n", s.Image)
			resp, err := cli.ImagePull(ctx, s.Image, types.ImagePullOptions{})
			if err != nil {
				return 0, fmt.Errorf("can't pull docker image. %v", err)
			}
			defer resp.Close()
			decoder := json.NewDecoder(resp)
			for {
				var msg message.JSONMessage
				err := decoder.Decode(&msg)
				if err == io.EOF {
					// Completed
					break
				}
				if err != nil {
					return 0, fmt.Errorf("failed to pull docker image. %v", err)
				}
				if msg.Error != nil {
					return 0, fmt.Errorf("failed to pull docker image. %v", msg.Error)
				}
				fmt.Println(msg.ProgressMessage)
			}
		}

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
