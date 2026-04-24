# Metrics Computation in the Simulator

This document describes how each metric is computed within the llm-d-inference-sim simulator.

## Overview

The simulator tracks various metrics to provide insights into inference performance, resource utilization, and system behavior. Metrics are computed in real-time as requests are processed and exposed via the `/metrics` endpoint in Prometheus format.

## Latency Metrics

### Time To First Token (TTFT)

**Metric:** `vllm:time_to_first_token_seconds`

**Computation Location:** `pkg/llm-d-inference-sim/latencies.go`

The TTFT calculation depends on the configured latency calculator type:

#### Default Calculator
Automatically chooses between constant and per-token calculation based on configuration:

**For Remote Prefill (Disaggregated):**
```
If KVCacheTransferTimePerToken is configured:
    TTFT = KVCacheTransferTimePerToken × PromptTokens × GPUSaturationFactor
    
If KVCacheTransferLatency is configured:
    TTFT = KVCacheTransferLatency × GPUSaturationFactor
```

**For Local Prefill (Aggregated):**
```
If PrefillTimePerToken is configured:
    TTFT = (PrefillOverhead + (PromptTokens - CachedPromptTokens) × PrefillTimePerToken) 
           × LoadFactor × GPUSaturationFactor
    
If TimeToFirstToken is configured:
    TTFT = TimeToFirstToken × LoadFactor × GPUSaturationFactor
```

#### Constant Calculator
Uses fixed latency values regardless of prompt size:
```
For Remote Prefill:
    TTFT = KVCacheTransferLatency × GPUSaturationFactor

For Local Prefill:
    TTFT = TimeToFirstToken × LoadFactor × GPUSaturationFactor
```

#### Per Token Calculator
Always uses token-based calculation:
```
For Remote Prefill:
    TTFT = KVCacheTransferTimePerToken × PromptTokens × GPUSaturationFactor

For Local Prefill:
    TTFT = (PrefillOverhead + (PromptTokens - CachedPromptTokens) × PrefillTimePerToken)
           × LoadFactor × GPUSaturationFactor
```

**Load Factor Calculation:**
```
LoadFactor = 1 + (TimeFactorUnderLoad - 1) × (RunningReqs - 1) / (MaxNumSeqs - 1)
```
Where:
- `TimeFactorUnderLoad`: Configured multiplier for maximum load
- `RunningReqs`: Current number of running requests
- `MaxNumSeqs`: Maximum number of sequences that can run concurrently

**Random Variation:**
All TTFT values have random variation applied using normal distribution:
```
FinalTTFT = RandomNormal(TTFT, TimeToFirstTokenStdDev)
```

### Inter-Token Latency (ITL)

**Metric:** `vllm:inter_token_latency_seconds`

**Computation Location:** `pkg/llm-d-inference-sim/latencies.go`

```
ITL = InterTokenLatency × LoadFactor × GPUSaturationFactor
FinalITL = RandomNormal(ITL, InterTokenLatencyStdDev)
```

Where:
- `InterTokenLatency`: Base latency from configuration
- `LoadFactor`: Same calculation as TTFT
- `GPUSaturationFactor`: GPU utilization penalty (see below)
- `InterTokenLatencyStdDev`: Standard deviation for random variation

### GPU Saturation Factor

**Computation Location:** `pkg/llm-d-inference-sim/latencies.go` (`getGPUSaturationFactor`)

The GPU saturation factor adds latency penalties based on GPU thread utilization:

```
UtilizationRatio = CurrentUtilization / UtilizationCap

If UtilizationRatio < 0.8:
    GPUSaturationFactor = 1.0  (no penalty)

If 0.8 ≤ UtilizationRatio < 1.0:
    GPUSaturationFactor = 1.0 + (UtilizationRatio - 0.8) × 2.5
    (Linear increase from 1.0 to 1.5)

If UtilizationRatio ≥ 1.0:
    ExcessRatio = UtilizationRatio - 1.0
    GPUSaturationFactor = 1.5 + ExcessRatio × 5.0
    (Exponential increase for over-utilization)
```

**Example Values:**
- 70% utilization: 1.0× (no penalty)
- 90% utilization: 1.25×
- 100% utilization: 1.5×
- 110% utilization: 2.0×
- 120% utilization: 2.5×

### End-to-End Request Latency

**Metric:** `vllm:e2e_request_latency_seconds`

**Computation Location:** `pkg/llm-d-inference-sim/worker.go`

```
E2ELatency = CurrentTime - RequestStartTime
```

Measured from when the request enters the system until the response is fully generated. Includes:
- Queue time (waiting for worker)
- Prefill time (processing prompt)
- Decode time (generating tokens)

**Code:**
```go
common.WriteToChannel(s.Context.metrics.e2eReqLatencyChan, 
    time.Since(reqCtx.startProcessingTime()).Seconds(), s.Context.logger)
```

### Request Inference Time

**Metric:** `vllm:request_inference_time_seconds`

**Computation Location:** `pkg/llm-d-inference-sim/worker.go`

```
InferenceTime = CurrentTime - ProcessingStartTime
```

Time spent in the RUNNING phase, excluding queue time:

**Code:**
```go
startTime := time.Now()
// ... process request ...
common.WriteToChannel(s.Context.metrics.reqInferenceTimeChan, 
    time.Since(startTime).Seconds(), s.Context.logger)
```

### Request Queue Time

**Metric:** `vllm:request_queue_time_seconds`

Time spent in WAITING phase before a worker becomes available:
```
QueueTime = ProcessingStartTime - RequestArrivalTime
```

### Request Prefill Time

**Metric:** `vllm:request_prefill_time_seconds`

Time spent processing the prompt tokens (prefill phase):
```
PrefillTime = FirstTokenGenerationTime - ProcessingStartTime
```

### Request Decode Time

**Metric:** `vllm:request_decode_time_seconds`

Time spent generating output tokens (decode phase):
```
DecodeTime = RequestCompletionTime - FirstTokenGenerationTime
```

### Time Per Output Token

**Metric:** `vllm:time_per_output_token_seconds`

Average time to generate each output token:
```
TimePerOutputToken = DecodeTime / CompletionTokens
```

## Token Metrics

### Prompt Tokens

**Metric:** `vllm:request_prompt_tokens`

**Computation:** Counted during tokenization of the input prompt using the configured tokenizer.

**Histogram:** Tracks distribution of prompt token counts across requests.

### Generation Tokens

**Metric:** `vllm:request_generation_tokens`

**Computation:** Number of tokens generated in the response, determined by:
- Reaching `max_tokens` limit
- Encountering stop sequence
- Model-specific end-of-sequence token

**Histogram:** Tracks distribution of generation token counts.

### Max Generation Tokens

**Metric:** `vllm:max_num_generation_tokens`

**Computation:** Maximum number of tokens requested for generation. Currently same as `request_generation_tokens` since only one choice is returned per request.

### Request Max Tokens Parameter

**Metric:** `vllm:request_params_max_tokens`

**Computation:** Direct value from the `max_tokens` parameter in the request.

**Histogram:** Tracks distribution of requested max token values.

### Total Prompt Tokens

**Metric:** `vllm:prompt_tokens_total`

**Computation:** Cumulative counter incremented by prompt token count for each request:
```
prompt_tokens_total += request.PromptTokens
```

### Total Generation Tokens

**Metric:** `vllm:generation_tokens_total`

**Computation:** Cumulative counter incremented by generation token count for each request:
```
generation_tokens_total += request.CompletionTokens
```

## Request State Metrics

### Running Requests

**Metric:** `vllm:num_requests_running`

**Computation:** Real-time gauge tracking active requests:
```
On request start: num_requests_running++
On request complete: num_requests_running--
```

### Waiting Requests

**Metric:** `vllm:num_requests_waiting`

**Computation:** Real-time gauge tracking queued requests:
```
On request enqueue: num_requests_waiting++
On request dequeue: num_requests_waiting--
```

### Request Success Total

**Metric:** `vllm:request_success_total`

**Computation:** Counter incremented for each successfully completed request.

**Code Location:** `pkg/llm-d-inference-sim/worker.go`
```go
common.WriteToChannel(s.Context.metrics.requestSuccessChan,
    requestSuccessEvent{
        promptTokens:     respCtx.UsageData().PromptTokens,
        generationTokens: respCtx.UsageData().CompletionTokens,
        genTokensPerChoice: []int{respCtx.UsageData().CompletionTokens},
        maxTokens:          req.GetMaxCompletionTokens(),
        finishReason:       *respCtx.FinishReason()},
    s.Context.logger)
```

## KV Cache Metrics

### KV Cache Usage Percentage

**Metric:** `vllm:kv_cache_usage_perc`

**Computation Location:** `pkg/kv-cache/kv_cache.go`

```
KVCacheUsage = UsedBlocks / TotalBlocks
```

Where:
- `UsedBlocks`: Number of cache blocks currently allocated to requests
- `TotalBlocks`: Total number of cache blocks available

**Range:** 0.0 to 1.0 (0% to 100%)

**Updates:** Recalculated whenever requests allocate or free cache blocks.

### Prefix Cache Hits

**Metric:** `vllm:prefix_cache_hits`

**Computation:** Number of tokens successfully reused from the prefix cache.

When a new request arrives:
1. Check if prompt prefix matches existing cached prefixes
2. If match found, count the number of matching tokens
3. Increment `prefix_cache_hits` by the number of cached tokens

### Prefix Cache Queries

**Metric:** `vllm:prefix_cache_queries`

**Computation:** Total number of tokens queried against the prefix cache (includes both hits and misses).

For each request:
```
prefix_cache_queries += PromptTokens
```

### Cache Config Info

**Metric:** `vllm:cache_config_info`

**Computation:** Static information about the KV cache configuration:
- Block size
- Number of GPU blocks
- Number of CPU blocks
- Cache type (e.g., "auto", "prefix")

## GPU Resource Metrics

The simulator uses a **queue theory-based resource estimator** that provides accurate GPU resource consumption estimates based on model architecture, GPU specifications, and workload characteristics.

### GPU Active Thread Percentage

**Metric:** `vllm:gpu_active_thread_percentage`

**Computation Location:** `pkg/llm-d-inference-sim/resource_calculator.go`

The thread utilization is calculated using queue theory principles that model prefill and decode phases:

```
PrefillThreads = PrefillTokens × MinThreads
DecodeThreads = EffectiveBatchSize × MinThreads
TotalActiveThreads = PrefillThreads + DecodeThreads

ThreadOccupancy = (TotalActiveThreads / (GPUMaxThreads × NumGPUs)) × 100
ThreadUtilizationPerGPU = ThreadOccupancy / NumGPUs

ThreadUtilizationPerGPU = min(ThreadUtilizationPerGPU, GPUThreadLimit)
```

Where:
- `PrefillTokens`: Tokens that need computation (PromptTokens × (1 - CachedHitRatio) × EffectiveBatchSize)
- `MinThreads`: Minimum threads to saturate hidden dimension (equals model's HiddenSize)
- `EffectiveBatchSize`: Number of sequences being processed (typically MaxNumSeqs)
- `GPUMaxThreads`: Hardware thread capacity (e.g., 184,320 for H100, 108,544 for A100)
- `NumGPUs`: Number of GPU devices
- `GPUThreadLimit`: Maximum thread utilization from environment

**GPU Specifications:**
```
GPU Model                  MaxThreads    VRAM    Memory BW
NVIDIA-H200-SXM5-141GB     249,856      141 GB   4.8 TB/s
NVIDIA-H100-SXM5-80GB      184,320       80 GB   3.35 TB/s
NVIDIA-A100-SXM4-80GB      108,544       80 GB   2.0 TB/s
NVIDIA-L4-24GB              30,720       24 GB   0.3 TB/s
```

**Range:** 0 to 100 (percentage)

### GPU Memory Usage

**Metric:** `vllm:gpu_memory_usage_bytes`

**Computation Location:** `pkg/llm-d-inference-sim/resource_calculator.go`

Memory usage is calculated using detailed model architecture parameters:

```
ModelWeight = ModelSize / NumGPUs
KVCacheMem = (EffectiveBatchSize × TotalTokens × MemoryPerToken) / NumGPUs / 1e9
ActivationMem = ActiveMemoryPerPrefillToken × PrefillTokens / NumGPUs / 1e9
AttentionMem = EstimateAttentionMemory(PromptLen, PrefillTokens) / NumGPUs

TotalMemoryPerGPU = (ModelWeight + KVCacheMem + ActivationMem + AttentionMem) × MemoryOverheadFactor

TotalMemoryPerGPU = min(TotalMemoryPerGPU, GPUMemoryLimit)
```

Where:
- `ModelSize`: Estimated from model parameters (ParametersB × Precision)
- `MemoryPerToken`: KV cache memory = 2 × Layers × KVHeads × HeadDim × Precision
- `ActiveMemoryPerPrefillToken`: Activation memory = (HiddenSize + 4×HiddenSize) × Precision × EffectiveBatch
- `AttentionMem`: Additional memory for long contexts (>8K tokens), scales sublinearly
- `MemoryOverheadFactor`: Framework overhead (default: 1.2 = 20%)
- `GPUMemoryLimit`: Maximum memory from environment

**Model Architecture Parameters:**

Known model families with inferred parameters:
```
Model Family        Parameters  Layers  KVHeads  HiddenSize  QueryHeads  Precision
llama-3-8b          8.0B        32      8        4096        32          2 bytes
llama-3.1-8b        8.0B        32      8        4096        32          2 bytes
llama-3-70b         70.0B       80      8        8192        64          2 bytes
mistral-nemo        12.0B       40      8        5120        32          2 bytes
phi-3-mini          3.8B        32      8        3072        32          2 bytes
phi-3-medium        14.0B       40      8        5120        40          2 bytes
```

For unknown models, parameters are inferred from the model name using regex patterns (e.g., "7b", "70b").

**Model Size Estimation:**
```
Model Name Pattern → Estimated Size (FP16, 2 bytes/param)
"1b"              → 2 GB
"3b"              → 6 GB
"7b" or "8b"      → 14-16 GB
"13b"             → 26 GB
"34b"             → 68 GB
"70b"             → 140 GB
"175b"            → 350 GB
"405b"            → 810 GB
```

**Precision Detection:**
```
Model Name Pattern → Precision
"fp8" or "int8"   → 1 byte
"fp16" or "bf16"  → 2 bytes (default)
"awq" or "gptq"   → 2 bytes (KV cache still 16-bit)
```

**Attention Memory for Long Contexts:**

For sequences > 8K tokens, FlashAttention reduces memory from O(n²) to O(n), but some overhead remains:
```
If SequenceLength ≤ 8192:
    AttentionMem = 0

If SequenceLength > 8192:
    LongContextFactor = (SequenceLength / 8192)^1.3
    AttentionMem = PrefillTokens × HiddenSize × Precision × LongContextFactor × 0.1 / 1e9
```

### GPU Compute Allocation Percentage

**Metric:** `vllm:gpu_compute_allocation_percentage`

**Computation:** Static limit read from environment variable:
```
NVIDIA_MIG_COMPUTE_PERCENTAGE_{device_id}
```

**Range:** 0 to 100 (percentage)

This represents the GPU compute allocation limit for the device (e.g., MIG slice allocation).

### GPU Memory Allocation Bytes

**Metric:** `vllm:gpu_memory_allocation_bytes`

**Computation:** Static limit read from environment variable:
```
NVIDIA_MIG_MEMORY_LIMIT_BYTES_{device_id}
```

This represents the GPU memory allocation limit for the device in bytes.

## LoRA Metrics

### LoRA Requests Info

**Metric:** `vllm:lora_requests_info`

**Computation:** Tracks active LoRA adapters and their usage.

**Structure:**
```
lora_requests_info{lora_name="adapter1"} = reference_count
```

Where `reference_count` is the number of currently running requests using that LoRA adapter.

**Updates:**
- On request start with LoRA: Increment reference count
- On request complete with LoRA: Decrement reference count

## Metric Collection Architecture

### Histogram Metrics

All histogram metrics (latencies, token counts) use Prometheus histogram buckets:

**Tracked Values:**
- `_sum`: Sum of all observed values
- `_count`: Total number of observations
- `_bucket{le="x"}`: Count of observations ≤ x

**Average Calculation:**
```
Average = _sum / _count
```

**Percentiles:** Calculated from bucket distributions

### Counter Metrics

Counter metrics (totals, success counts) increment monotonically:

**Tracked Values:**
- Current total count
- Rate of change (calculated by Prometheus)

### Gauge Metrics

Gauge metrics (running requests, cache usage) represent current state:

**Updates:** Set to current value whenever state changes

### Channel-Based Collection

Most metrics are collected via Go channels to avoid blocking request processing:

```go
common.WriteToChannel(s.Context.metrics.metricChan, value, logger)
```

A dedicated goroutine reads from these channels and updates Prometheus metrics asynchronously.

## Configuration Parameters

Key configuration parameters affecting metric computation:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `inter-token-latency` | 10ms | Base inter-token latency |
| `inter-token-latency-std-dev` | 0ms | ITL standard deviation |
| `time-to-first-token` | 100ms | Fixed TTFT (constant mode) |
| `time-to-first-token-std-dev` | 0ms | TTFT standard deviation |
| `prefill-overhead` | 0ms | Fixed prefill startup cost |
| `prefill-time-per-token` | 0ms | Per-token prefill time |
| `prefill-time-std-dev` | 0ms | Prefill time standard deviation |
| `kv-cache-transfer-latency` | 0ms | Fixed KV transfer time |
| `kv-cache-transfer-time-per-token` | 0ms | Per-token KV transfer time |
| `time-factor-under-load` | 1.0 | Load multiplier at max capacity |
| `max-num-seqs` | 256 | Maximum concurrent sequences |
| `latency-calculator` | "default" | Calculator type (default/constant/per-token) |

## Example Calculations

### Example 1: Simple Request

**Configuration:**
- `prefill-overhead`: 50ms
- `prefill-time-per-token`: 2ms
- `inter-token-latency`: 25ms
- `max-num-seqs`: 256
- `time-factor-under-load`: 2.0

**Request:**
- Prompt: "Hello" (1 token)
- Max tokens: 100
- Running requests: 1

**Calculations:**
```
LoadFactor = 1 + (2.0 - 1) × (1 - 1) / (256 - 1) = 1.0

TTFT = (50ms + (1 - 0) × 2ms) × 1.0 × 1.0 = 52ms

ITL = 25ms × 1.0 × 1.0 = 25ms

TotalDecodeTime = 100 × 25ms = 2,500ms = 2.5s

E2ELatency = 52ms + 2,500ms = 2,552ms ≈ 2.55s
```

### Example 2: High Load with GPU Saturation

**Configuration:** Same as Example 1

**Request:**
- Prompt: 100 tokens
- Max tokens: 50
- Running requests: 128 (50% of max)
- GPU utilization: 95% (cap: 100%)

**Calculations:**
```
LoadFactor = 1 + (2.0 - 1) × (128 - 1) / (256 - 1) = 1.498

UtilizationRatio = 95 / 100 = 0.95
GPUSaturationFactor = 1.0 + (0.95 - 0.8) × 2.5 = 1.375

TTFT = (50ms + 100 × 2ms) × 1.498 × 1.375 = 515ms

ITL = 25ms × 1.498 × 1.375 = 51.4ms

TotalDecodeTime = 50 × 51.4ms = 2,570ms = 2.57s

E2ELatency = 515ms + 2,570ms = 3,085ms ≈ 3.09s
```

## Verification

To verify metric computation:

1. **Check Prometheus metrics:**
```bash
curl http://localhost:8000/metrics | grep vllm
```

2. **Calculate averages:**
```bash
# Average TTFT
curl -s http://localhost:8000/metrics | grep "time_to_first_token_seconds" | \
  awk '/sum/ {sum=$2} /count/ {count=$2} END {printf "%.3f\n", sum/count}'
```

3. **Compare with configuration:**
Verify that observed metrics match expected values based on configuration and load.

## References

- **Code:** `pkg/llm-d-inference-sim/latencies.go` - Latency calculations
- **Code:** `pkg/llm-d-inference-sim/worker.go` - Request processing and timing
- **Code:** `pkg/llm-d-inference-sim/resource_calculator.go` - GPU resource estimation
- **Docs:** `docs/metrics.md` - Metric descriptions
- **Docs:** `docs/understanding-latency-metrics.md` - Latency measurement guide