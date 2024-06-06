package patientsummary

var (
	triggers = map[string]struct{}{
		"LEGACY_DATA_ADDED":       {},
		"LEGACY_UPLOAD_COMPLETED": {},
		"UPLOAD_COMPLETED":        {},
	}
	types = map[string]struct{}{
		"bgm": {},
		"cgm": {},
	}
)

func ShouldTriggerEHRSync[T Stats](s Summary[T]) bool {
	if s.Type == nil {
		return false
	}
	if _, ok := types[*s.Type]; !ok {
		return false
	}

	// After a summary recalculation last updated reason is not empty, but outdated reason is.
	// This is the only time we should consider triggering an EHR sync
	if s.Dates.LastUpdatedReason == nil || len(*s.Dates.LastUpdatedReason) == 0 {
		return false
	}
	if s.Dates.OutdatedReason != nil && len(*s.Dates.OutdatedReason) != 0 {
		return false
	}

	// Trigger sync only if upload is completed or if summary was recalculated during jellyfish upload session
	for _, typ := range *s.Dates.LastUpdatedReason {
		if _, ok := triggers[typ]; ok {
			return true
		}
	}

	return false
}
