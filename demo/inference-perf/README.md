# Demo with inference-perf

## Pre-requisite

- kind cluster from [kind-deploy.sh](../../kind-deploy.sh) with an example deployment
    > make dev-env-kind
- monitoring stack
  - Install Prometheus (and Grafana) with the node port 30909 for prometheus and 30300 for grafana (kind expose). Note that, the provided [node ports](../monitor/nodeports-observability.yaml) uses [knative monitoring stack installation](https://knative.dev/docs/serving/observability/metrics/collecting-metrics/).
  - Import [Grafana dashboard](https://github.com/vllm-project/vllm/blob/main/examples/online_serving/dashboards/grafana/performance_statistics.json) (optional)
- inference-perf
    > Please refer to [inference-perf](https://github.com/kubernetes-sigs/inference-perf) for installation instruction.

## Deploy inference server simulator

With the `make dev-env-kind` command, the default vllm server should be deployed.

Edit the configuration as needed.

Example config:

```yaml
      containers:
      - args:
        # Model Configuration
        - --model=meta-llama/Llama-3.1-8B-Instruct
        - --port=8000
        - --max-model-len=4096           # 4K context window
        - --max-num-seqs=10              # Max 10 concurrent requests
        
        # Latency Configuration (realistic for 8B model)
        - --latency-calculator=per-token
        - --prefill-overhead=20ms
        - --prefill-time-per-token=5ms
        - --prefill-time-std-dev=1ms
        - --inter-token-latency=10ms
        - --time-factor-under-load=1.8   # 80% slowdown at max load

        # KV Cache Simulation
        - --enable-kvcache=true
        - --kv-cache-size=64
        - --block-size=8
        - --global-cache-hit-threshold=0.8
```

To simulate with DRA request, check [Run simulator with DRA example driver](../dra/README.md).

## Run workload

```sh
inference-perf --config vllm-config-random.yml
```

Refer to the other available config below:

Name|Description
---|---
vllm-config-random.yml|Simple random synthetic prompt (10-100 tokens)
vllm-config-simple-trace.yml|Replay [simple trace](./trace-replay/trace.csv) traffic
vllm-config-azuree-trace.yml|Replay [azure public data trace*](./trace-replay/AzureLMMInferenceTrace_multimodal.csv) traffic

[* Azure public data source](https://github.com/Azure/AzurePublicDataset/blob/master/AzureLMMInferenceDataset2025.md)

```text
@inproceedings{qlm2024patke,
  author = {Qiu, Haoran and Biswas, Anish and Zhao, Zihan and Mohan, Jayashree and Khare, Alind and Choukse, Esha and Goiri, {\'I}{\~n}igo and Zhang, Zeyu and Shen, Haiying and Bansal, Chetan and Ramjee, Ramachandran and Fonseca, Rodrigo},
  title = {ModServe: Modality- and Stage-Aware Resource Disaggregation for Scalable Multimodal Model Serving},
  year = {2025},
  publisher = {Association for Computing Machinery},
  address = {New York, NY, USA},
  booktitle = {Proceedings of the 2025 ACM Symposium on Cloud Computing (SoCC 2025)},
  location = {Virtual},
}
```

Check metrics in report or directly from [http://localhost:30909/query](http://localhost:30909/query).
