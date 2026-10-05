package config

// OfflineSyncConfig controls machine offline_sequence replay semantics.
type OfflineSyncConfig struct {
	// GapTolerant accepts offline_sequence > last_sequence+1 (skips missing slots) instead of Aborted.
	GapTolerant bool
}

func loadOfflineSyncConfig() OfflineSyncConfig {
	return OfflineSyncConfig{
		GapTolerant: getenvBool("MACHINE_OFFLINE_SEQUENCE_GAP_TOLERANT", true),
	}
}
