/*
Copyright 2025 The llm-d-inference-sim Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package llmdinferencesim

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/llm-d/llm-d-inference-sim/pkg/common"
)

// GPUResourceLimits holds the resource limits for a single GPU device
type GPUResourceLimits struct {
	// DeviceID is the GPU device identifier (e.g., 0, 1, 2, etc.)
	DeviceID int
	// ActiveThreadPercentage is the percentage of GPU threads that can be actively used (0-100)
	ActiveThreadPercentage float64
	// MemoryLimitBytes is the memory limit in bytes
	MemoryLimitBytes int64
}

// ResourceConsumption represents the estimated resource consumption for a request
type ResourceConsumption struct {
	// ActiveThreadPercentage is the estimated percentage of active GPU threads used (0-100)
	ActiveThreadPercentage float64
	// MemoryUsageBytes is the estimated memory usage in bytes
	MemoryUsageBytes int64
	// DeviceID is the GPU device this consumption applies to
	DeviceID int
}

// ResourceCalculator estimates GPU resource consumption based on input and model
type ResourceCalculator interface {
	// CalculateResourceConsumption estimates resource consumption for a request
	// Returns a slice of ResourceConsumption, one for each GPU device
	CalculateResourceConsumption(params *ResourceParams) []ResourceConsumption
	// GetGPULimits returns the configured GPU resource limits
	GetGPULimits() []GPUResourceLimits
}

// ResourceParams contains parameters for resource consumption calculation
type ResourceParams struct {
	// PromptTokens is the number of input tokens
	PromptTokens int
	// GenerationTokens is the number of tokens to generate
	GenerationTokens int
	// CachedPromptTokens is the number of cached prompt tokens
	CachedPromptTokens int
	// RunningReqs is the number of currently running requests
	RunningReqs int64
	// ModelName is the name of the model being used
	ModelName string
	// KVCacheUsagePercentage is the current KV cache utilization (0-1)
	KVCacheUsagePercentage float64
	// KVCacheSizeTokens is the total KV cache capacity in tokens
	KVCacheSizeTokens int
}

type defaultResourceCalculator struct {
	gpuLimits []GPUResourceLimits
	// Base memory per token in bytes (approximate)
	baseMemoryPerToken int64
	// Memory overhead for model and KV cache in bytes
	modelOverheadBytes int64
	// Thread utilization factor per token
	threadUtilizationPerToken float64
	// Base thread utilization percentage
	baseThreadUtilization float64
	// Model complexity factor (based on model size)
	modelComplexityFactor float64
}

// NewResourceCalculator creates a new resource calculator based on configuration
func NewResourceCalculator(config *common.Configuration) (ResourceCalculator, error) {
	gpuLimits, err := parseGPULimitsFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to parse GPU limits from environment: %w", err)
	}

	// If no GPU limits found in environment, return nil (resource tracking disabled)
	if len(gpuLimits) == 0 {
		return nil, nil
	}

	// Estimate base memory per token based on model size
	// This is a simplified estimation - in reality, it depends on model architecture
	// Typical values: ~2-4 bytes per token for KV cache per layer
	baseMemoryPerToken := int64(16) // bytes per token (conservative estimate)

	// Estimate model size (weights) based on model name
	// This includes model weights in FP16 precision
	modelOverheadBytes := estimateModelSize(config.Model)

	// Calculate model complexity factor based on model size
	// Larger models require more computation per token
	modelComplexityFactor := estimateModelComplexity(config.Model)

	return &defaultResourceCalculator{
		gpuLimits:                 gpuLimits,
		baseMemoryPerToken:        baseMemoryPerToken,
		modelOverheadBytes:        modelOverheadBytes,
		threadUtilizationPerToken: 0.1,                   // 0.1% per token (base)
		baseThreadUtilization:     5.0,                   // 5% base utilization
		modelComplexityFactor:     modelComplexityFactor, // Model size multiplier
	}, nil
}

// estimateModelSize estimates the model weight memory size based on model name
// Assumes FP16 precision (2 bytes per parameter)
func estimateModelSize(modelName string) int64 {
	modelNameLower := strings.ToLower(modelName)

	// Extract parameter count from model name
	// Common patterns: "7b", "8b", "13b", "70b", etc.

	// Check for specific model sizes
	if strings.Contains(modelNameLower, "405b") {
		return 405 * 1024 * 1024 * 1024 * 2 // 405B params × 2 bytes = 810 GB
	}
	if strings.Contains(modelNameLower, "175b") {
		return 175 * 1024 * 1024 * 1024 * 2 // 175B params × 2 bytes = 350 GB
	}
	if strings.Contains(modelNameLower, "70b") {
		return 70 * 1024 * 1024 * 1024 * 2 // 70B params × 2 bytes = 140 GB
	}
	if strings.Contains(modelNameLower, "65b") {
		return 65 * 1024 * 1024 * 1024 * 2 // 65B params × 2 bytes = 130 GB
	}
	if strings.Contains(modelNameLower, "34b") {
		return 34 * 1024 * 1024 * 1024 * 2 // 34B params × 2 bytes = 68 GB
	}
	if strings.Contains(modelNameLower, "33b") {
		return 33 * 1024 * 1024 * 1024 * 2 // 33B params × 2 bytes = 66 GB
	}
	if strings.Contains(modelNameLower, "13b") {
		return 13 * 1024 * 1024 * 1024 * 2 // 13B params × 2 bytes = 26 GB
	}
	if strings.Contains(modelNameLower, "8b") {
		return 8 * 1024 * 1024 * 1024 * 2 // 8B params × 2 bytes = 16 GB
	}
	if strings.Contains(modelNameLower, "7b") {
		return 7 * 1024 * 1024 * 1024 * 2 // 7B params × 2 bytes = 14 GB
	}
	if strings.Contains(modelNameLower, "3b") {
		return 3 * 1024 * 1024 * 1024 * 2 // 3B params × 2 bytes = 6 GB
	}
	if strings.Contains(modelNameLower, "1b") {
		return 1 * 1024 * 1024 * 1024 * 2 // 1B params × 2 bytes = 2 GB
	}

	// Default: assume 7B model if size not detected
	// Add 2GB overhead for framework, CUDA, etc.
	return (7 * 1024 * 1024 * 1024 * 2) + (2 * 1024 * 1024 * 1024) // 14 GB + 2 GB = 16 GB
}

// estimateModelComplexity estimates the computational complexity factor based on model size
// Larger models require more computation per token, affecting thread utilization
func estimateModelComplexity(modelName string) float64 {
	modelNameLower := strings.ToLower(modelName)

	// Complexity factor scales with model size
	// Baseline: 7B model = 1.0
	// Larger models require proportionally more computation

	if strings.Contains(modelNameLower, "405b") {
		return 6.0 // 405B is ~58x larger than 7B, but not linear scaling
	}
	if strings.Contains(modelNameLower, "175b") {
		return 4.5 // 175B is ~25x larger than 7B
	}
	if strings.Contains(modelNameLower, "70b") {
		return 3.0 // 70B is ~10x larger than 7B
	}
	if strings.Contains(modelNameLower, "65b") {
		return 2.8 // 65B is ~9x larger than 7B
	}
	if strings.Contains(modelNameLower, "34b") {
		return 2.0 // 34B is ~5x larger than 7B
	}
	if strings.Contains(modelNameLower, "33b") {
		return 2.0 // 33B is ~5x larger than 7B
	}
	if strings.Contains(modelNameLower, "13b") {
		return 1.4 // 13B is ~2x larger than 7B
	}
	if strings.Contains(modelNameLower, "8b") {
		return 1.1 // 8B is slightly larger than 7B
	}
	if strings.Contains(modelNameLower, "7b") {
		return 1.0 // 7B is baseline
	}
	if strings.Contains(modelNameLower, "3b") {
		return 0.6 // 3B is smaller than 7B
	}
	if strings.Contains(modelNameLower, "1b") {
		return 0.3 // 1B is much smaller than 7B
	}

	// Default: assume 7B model complexity
	return 1.0
}

// CalculateResourceConsumption estimates resource consumption for a request
// Returns a slice of ResourceConsumption, one for each GPU device, with resources
// distributed equally across all GPUs
func (r *defaultResourceCalculator) CalculateResourceConsumption(params *ResourceParams) []ResourceConsumption {
	if len(r.gpuLimits) == 0 {
		return []ResourceConsumption{}
	}

	numGPUs := len(r.gpuLimits)

	// Calculate total memory usage first
	// Memory = model overhead + request-specific memory + KV cache memory

	// Request-specific memory: (prompt tokens - cached tokens) * memory per token + generation tokens * memory per token
	effectivePromptTokens := params.PromptTokens - params.CachedPromptTokens
	totalTokens := effectivePromptTokens + params.GenerationTokens
	requestMemory := int64(totalTokens) * r.baseMemoryPerToken

	// KV cache memory: Calculate based on KV cache usage percentage and total capacity
	// KV cache stores key-value pairs for all tokens in the cache
	// Memory per token in KV cache is typically higher than base memory per token
	// because it stores both keys and values across all layers
	kvCacheMemory := int64(0)
	if params.KVCacheSizeTokens > 0 {
		// KV cache memory per token is approximately 2x base memory per token
		// (one for keys, one for values, across all layers)
		kvCacheMemoryPerToken := r.baseMemoryPerToken * 2
		kvCacheMemory = int64(float64(params.KVCacheSizeTokens) * params.KVCacheUsagePercentage * float64(kvCacheMemoryPerToken))
	}

	// Total memory = model overhead + request memory + KV cache memory
	totalMemoryUsage := r.modelOverheadBytes + requestMemory + kvCacheMemory

	// Calculate total thread utilization
	// Thread utilization increases with number of tokens, model complexity, and running requests
	totalThreadUtilization := r.baseThreadUtilization + float64(totalTokens)*r.threadUtilizationPerToken

	// Apply model complexity factor - larger models require more computation per token
	// This affects thread utilization as more GPU cores are engaged for larger models
	totalThreadUtilization *= r.modelComplexityFactor

	// Factor in KV cache pressure - higher cache usage increases thread utilization
	// as more memory bandwidth is needed for cache lookups
	if params.KVCacheUsagePercentage > 0.5 {
		// Add 5-15% thread utilization when cache is >50% full
		cachePressureFactor := 1.0 + ((params.KVCacheUsagePercentage - 0.5) * 0.3)
		totalThreadUtilization *= cachePressureFactor
	}

	// Factor in concurrent requests (more requests = higher utilization)
	if params.RunningReqs > 1 {
		concurrencyFactor := 1.0 + (float64(params.RunningReqs-1) * 0.15) // 15% increase per additional request
		totalThreadUtilization *= concurrencyFactor
	}

	// Distribute resources equally across all GPUs
	memoryPerGPU := totalMemoryUsage / int64(numGPUs)
	threadPerGPU := totalThreadUtilization / float64(numGPUs)

	// Create consumption entries for each GPU
	consumptions := make([]ResourceConsumption, numGPUs)
	for i, gpuLimit := range r.gpuLimits {
		memoryUsage := memoryPerGPU
		threadUtilization := threadPerGPU

		// Apply memory limit constraint for this GPU
		if memoryUsage > gpuLimit.MemoryLimitBytes {
			log.Printf("[ResourceCalculator] Memory usage %d bytes (%.2f GB) per GPU exceeds limit %d bytes (%.2f GB) (DeviceID: %d, TotalMemory: %d, NumGPUs: %d, PromptTokens: %d, GenerationTokens: %d)",
				memoryUsage, float64(memoryUsage)/(1024*1024*1024),
				gpuLimit.MemoryLimitBytes, float64(gpuLimit.MemoryLimitBytes)/(1024*1024*1024),
				gpuLimit.DeviceID, totalMemoryUsage, numGPUs,
				params.PromptTokens, params.GenerationTokens)

			memoryUsage = gpuLimit.MemoryLimitBytes
		}

		// Apply thread percentage limit constraint for this GPU
		if threadUtilization > gpuLimit.ActiveThreadPercentage {
			log.Printf("[ResourceCalculator] Thread utilization %.2f%% per GPU exceeds limit %.2f%% (DeviceID: %d, TotalThreads: %.2f%%, NumGPUs: %d, PromptTokens: %d, GenerationTokens: %d, RunningReqs: %d, KVCacheUsage: %.2f%%)",
				threadUtilization, gpuLimit.ActiveThreadPercentage, gpuLimit.DeviceID,
				totalThreadUtilization, numGPUs,
				params.PromptTokens, params.GenerationTokens, params.RunningReqs, params.KVCacheUsagePercentage*100)
			threadUtilization = gpuLimit.ActiveThreadPercentage
		}

		// Ensure thread utilization is within valid range
		if threadUtilization < 0 {
			log.Printf("[ResourceCalculator] Thread utilization %.2f%% is negative, capping to 0 (DeviceID: %d)", threadUtilization, gpuLimit.DeviceID)
			threadUtilization = 0
		}
		if threadUtilization > 100 {
			log.Printf("[ResourceCalculator] Thread utilization %.2f%% exceeds 100%%, capping to 100 (DeviceID: %d)", threadUtilization, gpuLimit.DeviceID)
			threadUtilization = 100
		}

		consumptions[i] = ResourceConsumption{
			ActiveThreadPercentage: threadUtilization,
			MemoryUsageBytes:       memoryUsage,
			DeviceID:               gpuLimit.DeviceID,
		}
	}

	return consumptions
}

// GetGPULimits returns the configured GPU resource limits
func (r *defaultResourceCalculator) GetGPULimits() []GPUResourceLimits {
	return r.gpuLimits
}

// parseGPULimitsFromEnv parses GPU resource limits from environment variables
// Expected format:
// GPU_DEVICE_<N>_ACTIVE_THREAD_PERCENTAGE=<percentage>
// GPU_DEVICE_<N>_MEMORY_LIMIT=<size with unit, e.g., 12Gi, 8GB>
func parseGPULimitsFromEnv() ([]GPUResourceLimits, error) {
	limits := make(map[int]*GPUResourceLimits)

	// Scan environment variables
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Check for GPU_DEVICE_<N>_ACTIVE_THREAD_PERCENTAGE
		if strings.HasPrefix(key, "GPU_DEVICE_") && strings.HasSuffix(key, "_ACTIVE_THREAD_PERCENTAGE") {
			deviceIDStr := strings.TrimPrefix(key, "GPU_DEVICE_")
			deviceIDStr = strings.TrimSuffix(deviceIDStr, "_ACTIVE_THREAD_PERCENTAGE")
			deviceID, err := strconv.Atoi(deviceIDStr)
			if err != nil {
				return nil, fmt.Errorf("invalid device ID in %s: %w", key, err)
			}

			percentage, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid percentage value in %s: %w", key, err)
			}

			if percentage < 0 || percentage > 100 {
				return nil, fmt.Errorf("percentage must be between 0 and 100 in %s", key)
			}

			if limits[deviceID] == nil {
				limits[deviceID] = &GPUResourceLimits{DeviceID: deviceID}
			}
			limits[deviceID].ActiveThreadPercentage = percentage
		}

		// Check for GPU_DEVICE_<N>_MEMORY_LIMIT
		if strings.HasPrefix(key, "GPU_DEVICE_") && strings.HasSuffix(key, "_MEMORY_LIMIT") {
			deviceIDStr := strings.TrimPrefix(key, "GPU_DEVICE_")
			deviceIDStr = strings.TrimSuffix(deviceIDStr, "_MEMORY_LIMIT")
			deviceID, err := strconv.Atoi(deviceIDStr)
			if err != nil {
				return nil, fmt.Errorf("invalid device ID in %s: %w", key, err)
			}

			memoryBytes, err := parseMemorySize(value)
			if err != nil {
				return nil, fmt.Errorf("invalid memory size in %s: %w", key, err)
			}

			if limits[deviceID] == nil {
				limits[deviceID] = &GPUResourceLimits{DeviceID: deviceID}
			}
			limits[deviceID].MemoryLimitBytes = memoryBytes
		}
	}

	// Convert map to slice and validate
	result := make([]GPUResourceLimits, 0, len(limits))
	for _, limit := range limits {
		// Validate that both fields are set
		if limit.ActiveThreadPercentage == 0 && limit.MemoryLimitBytes == 0 {
			return nil, fmt.Errorf("GPU device %d has no limits configured", limit.DeviceID)
		}
		result = append(result, *limit)
	}

	return result, nil
}

// parseMemorySize parses memory size strings like "12Gi", "8GB", "1024Mi", etc.
func parseMemorySize(size string) (int64, error) {
	size = strings.TrimSpace(size)
	if size == "" {
		return 0, fmt.Errorf("empty memory size")
	}

	// Find where the number ends and unit begins
	var numStr string
	var unit string
	foundUnit := false
	for i, ch := range size {
		if (ch < '0' || ch > '9') && ch != '.' {
			numStr = size[:i]
			unit = size[i:]
			foundUnit = true
			break
		}
	}

	// If no unit found, the entire string is the number
	if !foundUnit {
		numStr = size
		unit = ""
	}

	if numStr == "" {
		return 0, fmt.Errorf("no numeric value found in memory size: %s", size)
	}

	value, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric value: %w", err)
	}

	// Parse unit (case-insensitive)
	unit = strings.ToUpper(strings.TrimSpace(unit))
	var multiplier int64

	switch unit {
	case "B", "":
		multiplier = 1
	case "K", "KB":
		multiplier = 1000
	case "KI", "KIB":
		multiplier = 1024
	case "M", "MB":
		multiplier = 1000 * 1000
	case "MI", "MIB":
		multiplier = 1024 * 1024
	case "G", "GB":
		multiplier = 1000 * 1000 * 1000
	case "GI", "GIB":
		multiplier = 1024 * 1024 * 1024
	case "T", "TB":
		multiplier = 1000 * 1000 * 1000 * 1000
	case "TI", "TIB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown memory unit: %s", unit)
	}

	return int64(value * float64(multiplier)), nil
}
