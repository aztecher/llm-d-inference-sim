# llm-d Demo Helm Chart

This Helm chart (`llm-d-inference-sim-demo`) deploys demo scenarios for `llm-d-inference-sim`.
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

### DRA preset

The cluster must have the custom DRA example driver installed with the **vgpu-profile** branch:

> Source: https://github.com/sunya-ch/dra-example-driver/tree/vgpu-profile

Follow the [dra/README.md](../dra/README.md) for cluster and driver setup steps.

### PD-Disaggregation preset

The cluster must have the custom DRA example driver installed with the **v1-consumable-capacity** branch and Kubernetes v1.34+:

> Source: https://github.com/sunya-ch/dra-example-driver/tree/v1-consumable-capacity

Follow the [pd-disaggregation/README.md](../pd-disaggregation/README.md) for the full cluster and driver setup steps.

---

## Installation

### Deploy the DRA demo (default)

```sh
helm upgrade --install llm-d-inference-sim-demo ./demo/helm \
  --namespace default \
  --create-namespace
```

This deploys:
- `ResourceClaim/vllm-sim-with-dra-claim` — allocates a virtual GPU
- `Deployment/llm-d-inference-sim-demo` — simulator pod with the UDS tokenizer sidecar

Verify:
```sh
kubectl get pods,resourceclaims
```

### Deploy the PD-Disaggregation demo

Pick a scenario (1, 2, or 3 — see table below) and install:

```sh
helm upgrade --install llm-d-inference-sim-demo ./demo/helm \
  --namespace default \
  --create-namespace \
  --set preset=pd-disaggregation \
  --set pdDisaggregation.scenario=llama-only
```

#### Scenarios

| Scenario key | Deployments | Replicas |
|---|---|---|
| **`llama-only`** | `vllm-llama3-8b-instruct-prefill` + `vllm-llama3-8b-instruct-decode` | 2 prefill + 8 decode |
| **`granite-only`** | `vllm-granite-2b-prefill` + `vllm-granite-2b-decode` | 4 prefill + 24 decode |
| **`mixed`** | All four deployments above | Llama: 1+4 / Granite: 2+12 |

Verify:
```sh
kubectl get pods,resourceclaims
```

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
| `udsTokenizer.logLevel` | `"DEBUG"` | Log verbosity |
| `udsTokenizer.probePort` | `8082` | Health probe port |

### DRA preset

| Key | Default | Description |
|---|---|---|
| `dra.image.repository` | `ghcr.io/sunya-ch/llm-d-inference-sim` | Simulator image |
| `dra.image.tag` | `"v0.8.2-dirty"` | Image tag |
| `dra.model` | `meta-llama/Llama-3.1-8B-Instruct` | Model name |
| `dra.port` | `8000` | Simulator HTTP port |
| `dra.maxModelLen` | `4096` | Max context window |
| `dra.maxNumSeqs` | `10` | Max concurrent sequences |
| `dra.latency.*` | see values.yaml | Latency simulation parameters |
| `dra.kvCache.enabled` | `true` | Enable KV-cache simulation |
| `dra.hfToken` | `""` | Hugging Face token (optional) |
| `dra.resourceClaim.name` | `vllm-sim-with-dra-claim` | ResourceClaim name |
| `dra.resourceClaim.deviceClassName` | `vgpu.example.com` | DRA device class |
| `dra.resourceClaim.memory` | `"20Gi"` | GPU memory requested |
| `dra.resourceClaim.compute` | `"40"` | Compute units requested |

### PD-Disaggregation preset

| Key | Default | Description |
|---|---|---|
| `pdDisaggregation.scenario` | `"llama"` | Scenario key (`llama`, `granite`, `mixed`) |
| `pdDisaggregation.image.repository` | `ghcr.io/llm-d/llm-d-inference-sim` | Simulator image |
| `pdDisaggregation.image.tag` | `"latest"` | Image tag |
| `pdDisaggregation.port` | `8000` | Simulator HTTP port |
| `pdDisaggregation.maxLoras` | `2` | Max LoRA adapters |
| `pdDisaggregation.loraModules` | `[{"name":"food-review-1"}]` | LoRA module definitions |
| `pdDisaggregation.llama.*` | see values.yaml | Llama model and claim template config |
| `pdDisaggregation.granite.*` | see values.yaml | Granite model and claim template config |
| `pdDisaggregation.scenario3.*` | see values.yaml | Replica overrides for scenario 3 |

---

## Using a custom values file

```sh
# Copy and edit the values file
cp demo/helm/values.yaml my-values.yaml
# Edit my-values.yaml as needed, then install
helm upgrade --install llm-d-inference-sim-demo ./demo/helm \
  -f my-values.yaml
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
helm upgrade llm-d-inference-sim-demo ./demo/helm \
  --set preset=pd-disaggregation \
  --set pdDisaggregation.scenario=granite-only
```
