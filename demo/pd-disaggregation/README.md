# DEMO: Fine-grained prefill-decode disaggregation with DRA

This folder contains prefill-decode disaggregation deployment examples for demonstrating potentials of fine-grained device resource allocation using DRA consumable capacity feature.

## Prepare cluster with the custom example DRA driver

The custom example DRA driver is the driver that we customize the kubernetes-sigs/dra-example-driver to support alpha feature DRAConsumableCapacity on v1.34.0 release.

1. Clone `v1-consumable-capacity` from forked repo of kubernetes-sigs/dra-example-driver.

    ```sh
    git clone --branch v1-consumable-capacity --single-branch https://github.com/sunya-ch/dra-example-driver
    pushd dra-example-driver
    ```

2. Create Kind cluster with pre-built kindest/node image

    ```sh
    KIND_IMAGE=ghcr.io/sunya-ch/kindest/node:v1.34.0-rc.1 ./demo/create-cluster.sh
    ```

    > Default container tool is docker, please add `CONTAINER_TOOL=podman` to use podman.

    The following cluster should be created.

    ```
    $ kubectl get nodes
    NAME                                       STATUS   ROLES           AGE     VERSION
    dra-example-driver-cluster-control-plane   Ready    control-plane   4m34s   v1.34.0-rc.1
    dra-example-driver-cluster-worker          Ready    <none>          4m18s   v1.34.0-rc.1
    ```

3. Build and deploy example DRA driver image

    3.1. Build image

    ```sh
    ./demo/build-driver.sh
    ```

    > Default container tool is docker, please add `CONTAINER_TOOL=podman` to use podman.

    If the build is complete, the following message should be logged.
    
    ```
    Driver build complete: registry.k8s.io/dra-example-driver/dra-example-driver:v0.1.0
    ```

    3.2. Load image to Kind cluster

    ```sh
    ./demo/scripts/load-driver-image-into-kind.sh
    ```

    3.3. Deploy example DRA driver via helm chart

    ```sh
    helm upgrade -i \
      --create-namespace \
      --namespace dra-example-driver \
      dra-example-driver \
      deployments/helm/dra-example-driver
    ```

    The dra-driver should be running.

    ```
    $ kubectl get po -n dra-example-driver
    NAME                                     READY   STATUS    RESTARTS   AGE
    dra-example-driver-kubeletplugin-cjkrb   1/1     Running   0          7s
    ```

    And example resource slice should be created.

    ```
    $ kubectl get resourceslices
    NAME                                                      NODE                                DRIVER            POOL                                AGE
    dra-example-driver-cluster-worker-gpu.example.com-gz284   dra-example-driver-cluster-worker   gpu.example.com   dra-example-driver-cluster-worker   42s
    ```
    
    The ResourceSlice should contain 8 GPUs with the following allowMultipleAllocations flag and capacity declaration.

    ```yaml
    $ kubectl get $(kubectl get resourceslices -oname) -oyaml
    ...
    allowMultipleAllocations: true
    capacity:
      count:
        requestPolicy:
          default: "0"
          validValues:
          - "0"
          - "1"
        value: "1"
      memory:
        requestPolicy:
          default: 80Gi
          validRange:
            min: 1Gi
            step: 512Mi
        value: 80Gi
      thread:
        requestPolicy:
          default: "100"
          validRange:
            min: "1"
            step: "1"
        value: "100"
    ```

    > `count` capacity is used to forcefully have only one prefill per GPU device.

4. Move out to main workspace

    ```
    popd
    ```

## Build and load simulator image to Kind

1. Clone `llm-d-kubecon-na-2025` from the forked repo of llm-d/llm-d-inference-sim

    ```sh
    git clone --branch llm-d-kubecon-na-2025 --single-branch https://github.com/sunya-ch/llm-d-inference-sim.git
    pushd llm-d-inference-sim
    ```

2. Build image (requires go installed, being used for detecting CPU architecture)

    ```sh
    make image-build
    ```

3. Load image to kind

    ```sh
    CONTAINER_TOOL=docker
    DRIVER_IMAGE=ghcr.io/llm-d/llm-d-inference-sim:dev
    KIND_CLUSTER_NAME=dra-example-driver-cluster
    IMAGE_ARCHIVE=driver_image.tar
    ${CONTAINER_TOOL} save -o "${IMAGE_ARCHIVE}" "${DRIVER_IMAGE}" && \
    kind load image-archive \
        --name "${KIND_CLUSTER_NAME}" \
        "${IMAGE_ARCHIVE}"
    rm "${IMAGE_ARCHIVE}"
    ```

## Deploy each demonstrated use cases of p/d model services

Before deploying the deployment, deploy all resourceclaimtemplate in advance.

```sh
$ kubectl apply -f demo/pd-disaggregation/claim_template.yaml
```

```
resourceclaimtemplate.resource.k8s.io/llm-d-prefill-llama created
resourceclaimtemplate.resource.k8s.io/llm-d-decode-llama created
resourceclaimtemplate.resource.k8s.io/llm-d-prefill-granite created
resourceclaimtemplate.resource.k8s.io/llm-d-decode-granite created
```

### Deployment-1

- Llma-8b 2x(4TPx40Gi/70threads) Prefill
- Llam-8b 8x(40Gi/30threads) Decode

```sh
$ kubectl apply -f demo/pd-disaggregation/deployment-1.yaml
deployment.apps/vllm-llama3-8b-instruct-prefill created
deployment.apps/vllm-llama3-8b-instruct-decode created
# check all pods are running
$ kubectl get pods
NAME                                              READY   STATUS    RESTARTS   AGE
vllm-llama3-8b-instruct-decode-57d4645fbc-2k6nz   1/1     Running   0          3s
vllm-llama3-8b-instruct-decode-57d4645fbc-dckxm   1/1     Running   0          3s
vllm-llama3-8b-instruct-decode-57d4645fbc-djfr4   1/1     Running   0          3s
vllm-llama3-8b-instruct-decode-57d4645fbc-dv9rs   1/1     Running   0          3s
vllm-llama3-8b-instruct-decode-57d4645fbc-f7p8m   1/1     Running   0          3s
vllm-llama3-8b-instruct-decode-57d4645fbc-hqswk   1/1     Running   0          3s
vllm-llama3-8b-instruct-decode-57d4645fbc-nlb78   1/1     Running   0          3s
vllm-llama3-8b-instruct-decode-57d4645fbc-vxf2w   1/1     Running   0          3s
vllm-llama3-8b-instruct-prefill-578d4447-4thdh    1/1     Running   0          3s
vllm-llama3-8b-instruct-prefill-578d4447-vql5l    1/1     Running   0          3s
# check resourceclaims
$ kubectl get resourceclaims
NAME                                                           STATE                AGE
vllm-llama3-8b-instruct-decode-57d4645fbc-2k6-shared-gphc2jc   allocated,reserved   93s
vllm-llama3-8b-instruct-decode-57d4645fbc-dck-shared-gp6cfn9   allocated,reserved   93s
vllm-llama3-8b-instruct-decode-57d4645fbc-djf-shared-gpmwwp9   allocated,reserved   93s
vllm-llama3-8b-instruct-decode-57d4645fbc-dv9-shared-gpkmfvg   allocated,reserved   93s
vllm-llama3-8b-instruct-decode-57d4645fbc-f7p-shared-gpm8drv   allocated,reserved   93s
vllm-llama3-8b-instruct-decode-57d4645fbc-hqs-shared-gpxxxrj   allocated,reserved   93s
vllm-llama3-8b-instruct-decode-57d4645fbc-nlb-shared-gptf9x2   allocated,reserved   93s
vllm-llama3-8b-instruct-decode-57d4645fbc-vxf-shared-gp2b4sz   allocated,reserved   93s
vllm-llama3-8b-instruct-prefill-578d4447-4thd-shared-gpwdwxr   allocated,reserved   93s
vllm-llama3-8b-instruct-prefill-578d4447-vql5-shared-gpfvtwk   allocated,reserved   93s
```

Extra request should stay in pending state.

```
$ kubectl apply -f demo/pd-disaggregation/extra-request.yaml
deployment.apps/extra-vllm created
$ kubectl get pods
NAME                                              READY   STATUS    RESTARTS   AGE
extra-vllm-889c84944-s5jlk                        0/1     Pending   0          3s
...
```

Clean up
```
kubectl delete -f demo/pd-disaggregation/deployment-1.yaml
kubectl delete -f demo/pd-disaggregation/extra-request.yaml
```

### Deployment-2

- Granite-2b 4x(2x20Gi/40threads) Prefill
- Granite-2b  24x(20Gi/20threads) Decode

Repeat the similar steps to Deployment-1 with `demo/pd-disaggregation/deployment-2.yaml`

### Deployment-3

- Llma-8b 1x(4x40Gi/70threads) Prefill
- Llma-8b 4x(40Gi/30threads) Decode
- Granite-2b 2x(2x20Gi/40threads) Prefill
- Granite-2b 12x(20Gi/20threads) Decode

Repeat the similar steps to Deployment-1 with `demo/pd-disaggregation/deployment-3.yaml`

## Comparing to MIG Instance

Available MIG profile for A100-SXM4-80GB are 1g.10gb, 2g.20gb, 3g.40gb, 4g.40gb, and 7g.80gb.

- Llma-8b Prefill needs 4TP x 7g.80gb (compute bound)
- Llma-8b Decode needs at least 3g.40gb (memory bound)
- Granite-2b Prefill needs 2TP x 4g.40gb (compute bound)
- Granite-2b Decode needs at least 2g.20gb (memory bound)

For each scenarios,

- Deployment-1: 1x(4x7g.80gb) + 8x(3g.40gb)
- Deployment-2: 4x(2x4g.40gb) + 8x(3g.40gb) --> 2x2g.20gb cannot fit in, so 3g.40gb must be used
- Deployment-3: Llam [1x(4x7g.80gb) + 2x(3g.40gb)] + Granite [2x(2x4g.40gb) +  + 2x(3g.40gb)]
