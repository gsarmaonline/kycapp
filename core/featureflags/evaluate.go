package featureflags

import (
	"hash/fnv"
	"strings"
)

// Reasons returned by Evaluate.
const (
	ReasonDisabled    = "disabled"
	ReasonOverrideOn  = "override_include"
	ReasonOverrideOff = "override_exclude"
	ReasonPercentage  = "percentage"
	ReasonFull        = "full"
	ReasonOff         = "off"
)

// Bucket returns a sticky 0–99 bucket for flagKey + subjectID.
func Bucket(flagKey, subjectID string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(flagKey))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(subjectID))
	return int(h.Sum32() % 100)
}

// InRollout reports whether subjectID is inside the percentage bucket for flagKey.
// percentage is clamped to 0–100. An empty subjectID is never in rollout for 1–99.
func InRollout(flagKey, subjectID string, percentage int) bool {
	if percentage <= 0 {
		return false
	}
	if percentage >= 100 {
		return true
	}
	subjectID = strings.TrimSpace(subjectID)
	if subjectID == "" {
		return false
	}
	return Bucket(flagKey, subjectID) < percentage
}

// Evaluate applies kill switch → override → percentage.
// overrideEffect is "include", "exclude", or empty.
func Evaluate(enabled bool, percentage int, flagKey, subjectID, overrideEffect string) (enabledOut bool, reason string) {
	if !enabled {
		return false, ReasonDisabled
	}
	switch strings.TrimSpace(overrideEffect) {
	case "include":
		return true, ReasonOverrideOn
	case "exclude":
		return false, ReasonOverrideOff
	}
	if percentage >= 100 {
		return true, ReasonFull
	}
	if percentage <= 0 {
		return false, ReasonOff
	}
	if InRollout(flagKey, subjectID, percentage) {
		return true, ReasonPercentage
	}
	return false, ReasonPercentage
}
