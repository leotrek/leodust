package orbit

import (
	"fmt"
	"strings"
	"time"

	"github.com/leotrek/leodust/pkg/logging"
	"github.com/leotrek/leodust/pkg/types"
)

const (
	publishedReferencePositionToleranceKM       = 8.0
	publishedReferenceVelocityToleranceKMPerSec = 0.01
)

// ReferenceSample stores one published state vector used to validate propagation.
type ReferenceSample struct {
	Timestamp            time.Time
	ExpectedPositionKM   types.Vector
	ExpectedVelocityKMPS types.Vector
}

// ReferenceCase groups a TLE and the published state vectors that it should match.
type ReferenceCase struct {
	Name                  string
	Source                string
	Line1                 string
	Line2                 string
	PositionToleranceKM   float64
	VelocityToleranceKMPS float64
	Samples               []ReferenceSample
}

// ReferenceSampleResult contains the measured error for one published sample.
type ReferenceSampleResult struct {
	Timestamp            time.Time
	ExpectedPositionKM   types.Vector
	ActualPositionKM     types.Vector
	PositionErrorKM      float64
	ExpectedVelocityKMPS types.Vector
	ActualVelocityKMPS   types.Vector
	VelocityErrorKMPS    float64
}

// Passed reports whether the sample stayed within the configured tolerances.
func (r ReferenceSampleResult) Passed(positionToleranceKM, velocityToleranceKMPS float64) bool {
	return r.PositionErrorKM <= positionToleranceKM && r.VelocityErrorKMPS <= velocityToleranceKMPS
}

// ReferenceValidationResult summarizes one reference case.
type ReferenceValidationResult struct {
	Case                 ReferenceCase
	Samples              []ReferenceSampleResult
	MaxPositionErrorKM   float64
	MaxVelocityErrorKMPS float64
}

// Passed reports whether all samples stayed within the configured tolerances.
func (r ReferenceValidationResult) Passed() bool {
	return r.MaxPositionErrorKM <= r.Case.PositionToleranceKM &&
		r.MaxVelocityErrorKMPS <= r.Case.VelocityToleranceKMPS
}

// ValidateReferenceCase compares one TLE propagator against published reference samples.
func ValidateReferenceCase(referenceCase ReferenceCase) (ReferenceValidationResult, error) {
	propagator, err := NewTLEPropagator(referenceCase.Line1, referenceCase.Line2)
	if err != nil {
		return ReferenceValidationResult{}, err
	}

	result := ReferenceValidationResult{
		Case:    referenceCase,
		Samples: make([]ReferenceSampleResult, 0, len(referenceCase.Samples)),
	}

	for _, sample := range referenceCase.Samples {
		actualState, err := propagator.StateECI(sample.Timestamp)
		if err != nil {
			return ReferenceValidationResult{}, err
		}

		sampleResult := ReferenceSampleResult{
			Timestamp:            sample.Timestamp,
			ExpectedPositionKM:   sample.ExpectedPositionKM,
			ActualPositionKM:     actualState.PositionKM,
			PositionErrorKM:      vectorDistance(sample.ExpectedPositionKM, actualState.PositionKM),
			ExpectedVelocityKMPS: sample.ExpectedVelocityKMPS,
			ActualVelocityKMPS:   actualState.VelocityKMPerSec,
			VelocityErrorKMPS:    vectorDistance(sample.ExpectedVelocityKMPS, actualState.VelocityKMPerSec),
		}
		if sampleResult.PositionErrorKM > result.MaxPositionErrorKM {
			result.MaxPositionErrorKM = sampleResult.PositionErrorKM
		}
		if sampleResult.VelocityErrorKMPS > result.MaxVelocityErrorKMPS {
			result.MaxVelocityErrorKMPS = sampleResult.VelocityErrorKMPS
		}
		result.Samples = append(result.Samples, sampleResult)
	}

	return result, nil
}

// ValidatePublishedReferenceSuite runs the built-in published SGP4 reference case.
func ValidatePublishedReferenceSuite() ([]ReferenceValidationResult, error) {
	referenceCases := publishedReferenceCases()
	results := make([]ReferenceValidationResult, 0, len(referenceCases))
	failures := make([]string, 0)

	for _, referenceCase := range referenceCases {
		result, err := ValidateReferenceCase(referenceCase)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
		if !result.Passed() {
			failures = append(failures, fmt.Sprintf(
				"%s: max position error %.3f km (limit %.3f), max velocity error %.6f km/s (limit %.6f)",
				referenceCase.Name,
				result.MaxPositionErrorKM,
				referenceCase.PositionToleranceKM,
				result.MaxVelocityErrorKMPS,
				referenceCase.VelocityToleranceKMPS,
			))
		}
	}

	if len(failures) > 0 {
		return results, fmt.Errorf("published orbit validation failed: %s", strings.Join(failures, "; "))
	}

	return results, nil
}

// LogReferenceValidationResults prints a concise summary at info level and emits
// per-sample details either for failures or when debug logging is enabled.
func LogReferenceValidationResults(results []ReferenceValidationResult) {
	for _, result := range results {
		logging.Debugf("Orbit validation source for %s: %s", result.Case.Name, result.Case.Source)
		if result.Passed() {
			logging.Infof(
				"orbit validation passed for %s: max position error %.3f km, max velocity error %.6f km/s across %d samples",
				result.Case.Name,
				result.MaxPositionErrorKM,
				result.MaxVelocityErrorKMPS,
				len(result.Samples),
			)
		} else {
			logging.Errorf(
				"orbit validation failed for %s: max position error %.3f km, max velocity error %.6f km/s",
				result.Case.Name,
				result.MaxPositionErrorKM,
				result.MaxVelocityErrorKMPS,
			)
		}

		for _, sample := range result.Samples {
			if logging.Enabled(logging.DebugLevel) || !sample.Passed(result.Case.PositionToleranceKM, result.Case.VelocityToleranceKMPS) {
				logging.Debugf(
					"orbit sample %s at %s: position error %.3f km, velocity error %.6f km/s, expected pos=%+v actual pos=%+v",
					result.Case.Name,
					sample.Timestamp.UTC().Format(time.RFC3339Nano),
					sample.PositionErrorKM,
					sample.VelocityErrorKMPS,
					sample.ExpectedPositionKM,
					sample.ActualPositionKM,
				)
			}
		}
	}
}

func publishedReferenceCases() []ReferenceCase {
	return []ReferenceCase{
		{
			Name:                  "Vallado Case 00005 / WGS72",
			Source:                "Embedded from the published SGP4 regression vectors shipped in github.com/joshuaferrara/go-satellite",
			Line1:                 "1 00005U 58002B   00179.78495062  .00000023  00000-0  28098-4 0  4753",
			Line2:                 "2 00005  34.2682 348.7242 1859667 331.7664  19.3264 10.82419157413667",
			PositionToleranceKM:   publishedReferencePositionToleranceKM,
			VelocityToleranceKMPS: publishedReferenceVelocityToleranceKMPerSec,
			Samples: []ReferenceSample{
				referenceSample(
					"2000-06-28T00:50:19.733571Z",
					types.Vector{X: -7154.03120202, Y: -3783.17682504, Z: -3536.19412294},
					types.Vector{X: 4.741887409, Y: -4.151817765, Z: -2.093935425},
				),
				referenceSample(
					"2000-06-28T18:50:19.733571Z",
					types.Vector{X: -938.55923943, Y: -6268.18748831, Z: -4294.02924751},
					types.Vector{X: 7.536105209, Y: -0.427127707, Z: 0.989878080},
				),
				referenceSample(
					"2000-06-29T12:50:19.733571Z",
					types.Vector{X: 5579.55640116, Y: -3995.61396789, Z: -1518.82108966},
					types.Vector{X: 4.767927483, Y: 5.123185301, Z: 4.276837355},
				),
				referenceSample(
					"2000-06-30T06:50:19.733571Z",
					types.Vector{X: 6759.04583722, Y: 2001.58198220, Z: 2783.55192533},
					types.Vector{X: -2.180993947, Y: 6.402085603, Z: 3.644723952},
				),
				referenceSample(
					"2000-06-30T18:50:19.733571Z",
					types.Vector{X: -9060.47373569, Y: 4658.70952502, Z: 813.68673153},
					types.Vector{X: -2.232832783, Y: -4.110453490, Z: -3.157345433},
				),
			},
		},
	}
}

func referenceSample(timestamp string, positionKM, velocityKMPS types.Vector) ReferenceSample {
	return ReferenceSample{
		Timestamp:            mustParseReferenceTimestamp(timestamp),
		ExpectedPositionKM:   positionKM,
		ExpectedVelocityKMPS: velocityKMPS,
	}
}

func mustParseReferenceTimestamp(value string) time.Time {
	timestamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(err)
	}
	return timestamp
}

func vectorDistance(a, b types.Vector) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return types.Vector{X: dx, Y: dy, Z: dz}.Magnitude()
}
