package jira

import (
	"math"
	"slices"
	"time"
)

type cycleData map[string]time.Duration

type metrics struct {
	Average      string `json:"average"`
	Median       string `json:"median"`
	Percentile25 string `json:"percentile_25"`
	Percentile75 string `json:"percentile_75"`
}

func (d cycleData) Avg() time.Duration {
	sorted := d.sortedDurations()
	if len(sorted) == 0 {
		return 0
	}

	var sum time.Duration
	for _, dur := range sorted {
		sum += dur
	}
	return sum / time.Duration(len(sorted))
}

func (d cycleData) Median() time.Duration {
	return d.Percentile(50)
}

func (d cycleData) Percentile(p float64) time.Duration {
	sorted := d.sortedDurations()
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}

	rank := (p / 100) * float64(len(sorted)-1)
	lowerIdx := int(math.Floor(rank))
	upperIdx := int(math.Ceil(rank))
	frac := rank - float64(lowerIdx)

	upper := sorted[upperIdx]
	lower := sorted[lowerIdx]
	return lower + time.Duration(frac*float64(upper-lower))
}

func (d cycleData) sortedDurations() []time.Duration {
	var out []time.Duration
	for _, dur := range d {
		if dur > 0 {
			out = append(out, dur)
		}
	}

	slices.Sort(out)
	return out
}

func toIssueOutput(d cycleData) map[string]string {
	out := make(map[string]string)
	for k, v := range d {
		out[k] = v.String()
	}
	return out
}
