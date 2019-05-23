# About

Kitten is a local runner for Tekton CRD.
If you never heard about Tekton, check [here](https://tekton.dev/)

Kitten doesn't require a Kubernetes cluster to run
pipelines, it uses your (local) docker installation
to execute the exact same sequence of containers,
reusing some of the Tekton component, and just
replacing the Kubernetes CRD objects and Pod
scheduling with in-memory state and plain Docker
containers.

This project mostly started as an exercice to 
improve my Go skills. But I imagine it can be used
to run local tests of your Tekton pipeline.

This projet use raw Tekton CRDs, it doesn't define any higher level syntax 
to define your CI/CD process. We let other project work on this interesting
integration pattern (have a look at [Jenkins X](https://jenkinsx.io)).

Tekton logo is a (robot) cat, and any Kubernetes-related project name has to start with a `k`, so the name.

```
 A---A
{ O_O } 
  --- 
 (_^_)
```