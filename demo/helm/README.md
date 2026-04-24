# llm-d Demo Helm Chart

This Helm chart (`llm-d-inference-sim-demo`) deploys demo scenarios.
Two presets are available, selected via `preset` in `values.yaml`:

| Preset | Description |
|---|---|
| `dra` | Single simulator pod with a DRA vGPU ResourceClaim |
| `pd-disaggregation` | Prefill-decode disaggregated deployments backed by DRA consumable capacity |

---

## Prerequisites

### Common

- `helm` ≥ 3.x
- `kubectl` configured against a running cluster

### Setup custom DRA example driver

The cluster must have the custom DRA example driver installed with the **vgpu-profile** branch:

> Source: https://github.com/sunya-ch/dra-example-driver/tree/vgpu-profile

Follow the [dra/README.md](../dra/README.md) for cluster and driver setup steps.

or if you are using a Linux environment, you can simply install this using the following command

```bash
git clone -b vgpu-profile https://github.com/sunya-ch/dra-example-driver.git
cd dra-example-driver
helm upgrade --install \
  --create-namespace \
  --namespace dra-example-driver \
  dra-example-driver \
  deployments/helm/dra-example-driver \
  --set image.repository=ghcr.io/aztecher/dra-example-driver \
  --set image.tag=vgpu-profile
```


## Installation

### Deploy the DRA demo (default)

```sh
helm upgrade --install --namespace default llm-d-inference-sim-demo .
```

This deploys:
- `ResourceClaim/vllm-sim-with-dra-claim` — allocates a virtual GPU
- `Deployment/llm-d-inference-sim-demo` — simulator pod with the UDS tokenizer sidecar

When deploying to a Linux environment, please configure the image as follows:

```sh
helm upgrade --install \
  --namespace default \
  --create-namespace \
  llm-d-inference-sim-demo . \
  --set dra.image.repository=ghcr.io/aztecher/llm-d-inference-sim \
  --set dra.image.tag=dev
```

This deployment is the same as [../dra/vllm-sim-deploy.yaml](../dra/vllm-sim-deploy.yaml) but managed by helm.


### Deploy the PD-Disaggregation demo

Pick a scenario (`llama-only`, `granite-only` or `mixed` - see table below) and install:

```sh
helm upgrade --install \
  --namespace default \
  --create-namespace \
  llm-d-inference-sim-demo .  \
  --set preset=pd-disaggregation \
  --set pdDisaggregation.scenario=llama-only
```

#### Scenarios

| Scenario key | Deployments | Replicas (default) |
|---|---|---|
| **`llama-only`** | `vllm-llama3-8b-instruct-prefill` + `vllm-llama3-8b-instruct-decode` | 2 prefill + 8 decode |
| **`granite-only`** | `vllm-granite-2b-prefill` + `vllm-granite-2b-decode` | 4 prefill + 24 decode |
| **`mixed`** | All four deployments above | Llama: 1+4 / Granite: 2+12 |

You can change the number of prefillers and decoders of all scenarios by changing helm values.

This deployments are the same as [../pd-disaggregation](../pd-disaggregation) but managed by helm.

---

## Configuration

All parameters live in [`llm-d-inference-sim-demo/values.yaml`](./llm-d-inference-sim-demo/values.yaml).
Override any value with `--set key=value` or a custom values file.

### Top-level

| Key | Default | Description |
|---|---|---|
| `preset` | `"dra"` | Which demo to deploy (`dra` or `pd-disaggregation`) |

### UDS Tokenizer (DRA preset only)

| Key | Default | Description |
|---|---|---|
| `udsTokenizer.image` | `ghcr.io/llm-d/llm-d-uds-tokenizer:v0.6.0` | Tokenizer sidecar image |
| `udsTokenizer.imagePullPolicy` | `IfNotPresent` | Image pull policy |
| `udsTokenizer.logLevel` | `"DEBUG"` | Log verbosity |
| `udsTokenizer.probePort` | `8082` | Health probe port |

### DRA preset

| Key | Default | Description |
|---|---|---|
| `dra.image.repository` | `ghcr.io/sunya-ch/llm-d-inference-sim` | Simulator image |
| `dra.image.tag` | `"v0.8.2-dirty"` | Image tag |
| `dra.image.pullPolicy` | `IfNotPresent` | Image pull policy |
| `dra.model` | `meta-llama/Llama-3.1-8B-Instruct` | Model name |
| `dra.port` | `8000` | Simulator HTTP port |
| `dra.maxModelLen` | `4096` | Max context window |
| `dra.maxNumSeqs` | `10` | Max concurrent sequences |
| `dra.latency.*` | see values.yaml | Latency simulation parameters |
| `dra.kvCache.*` | see values.yaml | KV-cache simulation parameters |
| `dra.hfToken` | `""` | Hugging Face token (optional) |
| `dra.resourceClaimTemplate.name` | `vllm-sim-with-dra-claim` | ResourceClaimTemplate name |
| `dra.resourceClaimTemplate.deviceClassName` | `vgpu.example.com` | DRA device class |
| `dra.resourceClaimTemplate.memory` | `"20Gi"` | GPU memory requested |
| `dra.resourceClaimTemplate.compute` | `"40"` | Compute units requested |

### PD-Disaggregation preset

| Key | Default | Description |
|---|---|---|
| `pdDisaggregation.scenario` | `"llama-only"` | Scenario key (`llama-only`, `granite-only`, `mixed`) |
| `pdDisaggregation.image.repository` | `ghcr.io/llm-d/llm-d-inference-sim` | Simulator image |
| `pdDisaggregation.image.tag` | `"latest"` | Image tag |
| `pdDisaggregation.image.pullPolicy` | `IfNotPresent` | Image pull policy |
| `pdDisaggregation.port` | `8000` | Simulator HTTP port |
| `pdDisaggregation.maxLoras` | `2` | Max LoRA adapters |
| `pdDisaggregation.loraModules` | `[{"name":"food-review-1"}]` | LoRA module definitions |
| `pdDisaggregation.claimTemplates.*` | see values.yaml | DRA ResourceClaimTemplate spec |
| `pdDisaggregation.scenarios.*` | see values.yaml | Predefined specs for each scenario |

---

## Using a custom values file

```sh
# Copy and edit the values file
cp demo/helm/values.yaml my-values.yaml
# Edit my-values.yaml as needed, then install
helm upgrade --install llm-d-inference-sim-demo . -f my-values.yaml
```

---

## Uninstall

```sh
helm uninstall llm-d-inference-sim-demo
```

> ResourceClaims and ResourceClaimTemplates created by the chart are removed automatically.

---

## Switching between presets

To switch from one preset to another, simply upgrade with the new preset value.
Helm will remove resources that belong to the previous preset and create the new ones:

```sh
# Switch from DRA to pd-disaggregation granite scenario
helm upgrade llm-d-inference-sim-demo . \
  --set preset=pd-disaggregation \
  --set pdDisaggregation.scenario=granite-only
```

