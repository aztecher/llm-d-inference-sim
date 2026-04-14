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
	"time"

	"github.com/llm-d/llm-d-inference-sim/pkg/common"
)

type TTFTParams struct {
	PromptTokens         int
	CachedPromptTokens   int
	DoRemotePrefill      bool
	RunningReqs          int64
	ThreadUtilization    float64 // Current GPU thread utilization percentage (0-100)
	ThreadUtilizationCap float64 // GPU thread utilization limit/cap (0-100)
}

type InterTokenParams struct {
	RunningReqs          int64
	ThreadUtilization    float64 // Current GPU thread utilization percentage (0-100)
	ThreadUtilizationCap float64 // GPU thread utilization limit/cap (0-100)
}

type LatencyCalculator interface {
	// GetTimeToFirstToken returns time to first token. The simulator will wait
	// this amount of time before generating the first token.
	GetTimeToFirstToken(params *TTFTParams) time.Duration
	// GetInterTokenLatency returns inter-token latency. The simulator will wait
	// this amount of time before generating each response token (except the first one).
	GetInterTokenLatency(params *InterTokenParams) time.Duration
}

type baseCalculator struct {
	interTokenLatency       time.Duration
	interTokenLatencyStdDev time.Duration
	timeFactorUnderLoad     float64
	maxNumSeqs              int
	random                  *common.Random
}

// returns inter token latency
func (b *baseCalculator) GetInterTokenLatency(params *InterTokenParams) time.Duration {
	loadFactor := b.getCurrLoadFactor(params.RunningReqs)

	// Apply GPU saturation factor if thread utilization data is available
	gpuSaturationFactor := b.getGPUSaturationFactor(params.ThreadUtilization, params.ThreadUtilizationCap)

	latency := time.Duration(float64(b.interTokenLatency) * loadFactor * gpuSaturationFactor)
	return b.random.RandomNormDuration(latency, b.interTokenLatencyStdDev)
}

func (b *baseCalculator) getCurrLoadFactor(nRunningReqs int64) float64 {
	if b.maxNumSeqs <= 1 {
		return 1.0
	}
	return 1 + (b.timeFactorUnderLoad-1)*float64(nRunningReqs-1)/float64(b.maxNumSeqs-1)
}

// getGPUSaturationFactor calculates additional latency factor based on GPU utilization
// When GPU is saturated (utilization at or above cap), requests experience additional delays
func (b *baseCalculator) getGPUSaturationFactor(utilization, cap float64) float64 {
	// If no utilization data provided, return 1.0 (no adjustment)
	if cap <= 0 || utilization <= 0 {
		return 1.0
	}

	// Calculate utilization ratio (how close we are to the cap)
	utilizationRatio := utilization / cap

	// If under 80% of cap, no additional delay
	if utilizationRatio < 0.8 {
		return 1.0
	}

	// Between 80-100%: gradual increase in latency (up to 1.5x at cap)
	if utilizationRatio < 1.0 {
		// Linear increase from 1.0 to 1.5 as we go from 80% to 100%
		return 1.0 + (utilizationRatio-0.8)*2.5 // (1.0-0.8)*2.5 = 0.5, so max is 1.5
	}

	// At or above cap: significant performance degradation
	// Exponential increase: 1.5x at 100%, 2.0x at 110%, 3.0x at 120%, etc.
	if utilizationRatio >= 1.0 {
		excessRatio := utilizationRatio - 1.0
		return 1.5 + excessRatio*5.0 // Steep increase for over-utilization
	}

	return 1.0
}

// Default latency calculator. Decides whether to use per token or
// constant latency calculations based on the values of time-to-first-token
// and kv-cache-transfer-latency.
type defaultCalculator struct {
	baseCalculator
	timeToFirstToken             time.Duration
	timeToFirstTokenStdDev       time.Duration
	kVCacheTransferLatency       time.Duration
	kVCacheTransferLatencyStdDev time.Duration
	kVCacheTransferTimePerToken  time.Duration
	kVCacheTransferTimeStdDev    time.Duration
	prefillOverhead              time.Duration
	prefillTimePerToken          time.Duration
	prefillTimeStdDev            time.Duration
}

func newDefaultCalculator(config *common.Configuration, random *common.Random) *defaultCalculator {
	return &defaultCalculator{
		baseCalculator: baseCalculator{
			interTokenLatency:       config.InterTokenLatency.ToDuration(),
			interTokenLatencyStdDev: config.InterTokenLatencyStdDev.ToDuration(),
			timeFactorUnderLoad:     config.TimeFactorUnderLoad,
			maxNumSeqs:              config.MaxNumSeqs,
			random:                  random,
		},
		timeToFirstToken:             config.TimeToFirstToken.ToDuration(),
		timeToFirstTokenStdDev:       config.TimeToFirstTokenStdDev.ToDuration(),
		kVCacheTransferLatency:       config.KVCacheTransferLatency.ToDuration(),
		kVCacheTransferLatencyStdDev: config.KVCacheTransferLatencyStdDev.ToDuration(),
		kVCacheTransferTimePerToken:  config.KVCacheTransferTimePerToken.ToDuration(),
		kVCacheTransferTimeStdDev:    config.KVCacheTransferTimeStdDev.ToDuration(),
		prefillOverhead:              config.PrefillOverhead.ToDuration(),
		prefillTimePerToken:          config.PrefillTimePerToken.ToDuration(),
		prefillTimeStdDev:            config.PrefillTimeStdDev.ToDuration(),
	}
}

// returns time to first token
func (d *defaultCalculator) GetTimeToFirstToken(params *TTFTParams) time.Duration {
	loadFactor := d.getCurrLoadFactor(params.RunningReqs)
	gpuSaturationFactor := d.getGPUSaturationFactor(params.ThreadUtilization, params.ThreadUtilizationCap)

	if params.DoRemotePrefill {
		if d.kVCacheTransferLatency == 0 && d.kVCacheTransferLatencyStdDev == 0 {
			// is disaggregated PD and ttft is calculated using number of prompt tokens
			kvCacheTransT := time.Duration(float64(d.kVCacheTransferTimePerToken) * float64(params.PromptTokens) * gpuSaturationFactor)
			return d.random.RandomNormDuration(kvCacheTransT, d.kVCacheTransferTimeStdDev)
		}
		// is disaggregated PD and *not* using number of prompt tokens
		kvCacheLatency := time.Duration(float64(d.kVCacheTransferLatency) * gpuSaturationFactor)
		return d.random.RandomNormDuration(kvCacheLatency, d.kVCacheTransferLatencyStdDev)
	}
	if d.timeToFirstToken == 0 && d.timeToFirstTokenStdDev == 0 {
		// is aggregated PD and ttft is calculated using number of prompt tokens that are not in kv cache
		prefillOverhead := time.Duration(float64(d.prefillOverhead) * loadFactor * gpuSaturationFactor)
		prefillTimePerToken := time.Duration(float64(d.prefillTimePerToken) * loadFactor * gpuSaturationFactor)
		prefillTime := prefillOverhead + time.Duration(params.PromptTokens-params.CachedPromptTokens)*prefillTimePerToken
		return d.random.RandomNormDuration(prefillTime, d.prefillTimeStdDev)
	}

	// is aggregated PD and *not* using number of prompt tokens
	ttft := time.Duration(float64(d.timeToFirstToken) * loadFactor * gpuSaturationFactor)
	return d.random.RandomNormDuration(ttft, d.timeToFirstTokenStdDev)
}

// Constant latency calculator doesn't take the prompt size into account, uses
// time-to-first-token and kv-cache-transfer-latency and their std devs.
type constantCalculator struct {
	baseCalculator
	timeToFirstToken             time.Duration
	timeToFirstTokenStdDev       time.Duration
	kVCacheTransferLatency       time.Duration
	kVCacheTransferLatencyStdDev time.Duration
}

func newConstantCalculator(config *common.Configuration, random *common.Random) *constantCalculator {
	return &constantCalculator{
		baseCalculator: baseCalculator{
			interTokenLatency:       config.InterTokenLatency.ToDuration(),
			interTokenLatencyStdDev: config.InterTokenLatencyStdDev.ToDuration(),
			timeFactorUnderLoad:     config.TimeFactorUnderLoad,
			maxNumSeqs:              config.MaxNumSeqs,
			random:                  random,
		},
		timeToFirstToken:             config.TimeToFirstToken.ToDuration(),
		timeToFirstTokenStdDev:       config.TimeToFirstTokenStdDev.ToDuration(),
		kVCacheTransferLatency:       config.KVCacheTransferLatency.ToDuration(),
		kVCacheTransferLatencyStdDev: config.KVCacheTransferLatencyStdDev.ToDuration(),
	}
}

// returns time to first token
func (c *constantCalculator) GetTimeToFirstToken(params *TTFTParams) time.Duration {
	loadFactor := c.getCurrLoadFactor(params.RunningReqs)
	gpuSaturationFactor := c.getGPUSaturationFactor(params.ThreadUtilization, params.ThreadUtilizationCap)

	if params.DoRemotePrefill {
		// is disaggregated PD and *not* using number of prompt tokens
		kvCacheLatency := time.Duration(float64(c.kVCacheTransferLatency) * gpuSaturationFactor)
		return c.random.RandomNormDuration(kvCacheLatency, c.kVCacheTransferLatencyStdDev)
	}
	// is aggregated PD and *not* using number of prompt tokens
	ttft := time.Duration(float64(c.timeToFirstToken) * loadFactor * gpuSaturationFactor)
	return c.random.RandomNormDuration(ttft, c.timeToFirstTokenStdDev)
}

// Per token calculator takes the prompt size into account
type perTokenCalculator struct {
	baseCalculator
	kVCacheTransferTimePerToken time.Duration
	kVCacheTransferTimeStdDev   time.Duration
	prefillOverhead             time.Duration
	prefillTimePerToken         time.Duration
	prefillTimeStdDev           time.Duration
}

func newPerTokenCalculator(config *common.Configuration, random *common.Random) *perTokenCalculator {
	return &perTokenCalculator{
		baseCalculator: baseCalculator{
			interTokenLatency:       config.InterTokenLatency.ToDuration(),
			interTokenLatencyStdDev: config.InterTokenLatencyStdDev.ToDuration(),
			timeFactorUnderLoad:     config.TimeFactorUnderLoad,
			maxNumSeqs:              config.MaxNumSeqs,
			random:                  random,
		},
		kVCacheTransferTimePerToken: config.KVCacheTransferTimePerToken.ToDuration(),
		kVCacheTransferTimeStdDev:   config.KVCacheTransferTimeStdDev.ToDuration(),
		prefillOverhead:             config.PrefillOverhead.ToDuration(),
		prefillTimePerToken:         config.PrefillTimePerToken.ToDuration(),
		prefillTimeStdDev:           config.PrefillTimeStdDev.ToDuration(),
	}
}

// returns time to first token
func (p *perTokenCalculator) GetTimeToFirstToken(params *TTFTParams) time.Duration {
	loadFactor := p.getCurrLoadFactor(params.RunningReqs)
	gpuSaturationFactor := p.getGPUSaturationFactor(params.ThreadUtilization, params.ThreadUtilizationCap)

	if params.DoRemotePrefill {
		// is disaggregated PD and ttft is calculated using number of prompt tokens
		kvCacheTransT := time.Duration(float64(p.kVCacheTransferTimePerToken) * float64(params.PromptTokens) * gpuSaturationFactor)
		return p.random.RandomNormDuration(kvCacheTransT, p.kVCacheTransferTimeStdDev)
	}
	// is aggregated PD and ttft is calculated using number of prompt tokens that are not in kv cache
	prefillOverhead := time.Duration(float64(p.prefillOverhead) * loadFactor * gpuSaturationFactor)
	prefillTimePerToken := time.Duration(float64(p.prefillTimePerToken) * loadFactor * gpuSaturationFactor)
	prefillTime := prefillOverhead + time.Duration(params.PromptTokens-params.CachedPromptTokens)*prefillTimePerToken
	return p.random.RandomNormDuration(prefillTime, p.prefillTimeStdDev)
}
