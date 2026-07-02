package operator

import "strings"

// Canonical ended_reason values for machine_operator_sessions.
const (
	EndedReasonClientLogout              = "client_logout"
	EndedReasonHeartbeatTimeout          = "heartbeat_timeout"
	EndedReasonTechnicianTokenExpired    = "technician_token_expired"
	EndedReasonTokenRefreshFailed        = "token_refresh_failed"
	EndedReasonSupersededBySameOperator  = "superseded_by_same_operator"
	EndedReasonSupersededByAdminTakeover = "superseded_by_admin_takeover"
	EndedReasonAdminForcedClose          = "admin_forced_close"
	EndedReasonAppCrashDetected          = "app_crash_detected"
	EndedReasonDeviceReboot              = "device_reboot"
	EndedReasonSafeModeEntered           = "safe_mode_entered"
	EndedReasonCommissioningCompleted    = "commissioning_completed"
	EndedReasonSessionExpired            = "session_expired"
	EndedReasonServerRevoked             = "server_revoked"
	EndedReasonUnknown                   = "unknown"
)

var allowedEndedReasons = map[string]struct{}{
	EndedReasonClientLogout:              {},
	EndedReasonHeartbeatTimeout:          {},
	EndedReasonTechnicianTokenExpired:    {},
	EndedReasonTokenRefreshFailed:        {},
	EndedReasonSupersededBySameOperator:  {},
	EndedReasonSupersededByAdminTakeover: {},
	EndedReasonAdminForcedClose:          {},
	EndedReasonAppCrashDetected:          {},
	EndedReasonDeviceReboot:              {},
	EndedReasonSafeModeEntered:           {},
	EndedReasonCommissioningCompleted:    {},
	EndedReasonSessionExpired:            {},
	EndedReasonServerRevoked:             {},
	EndedReasonUnknown:                   {},
	EndedReasonStaleSessionReclaimed:     {},
	EndedReasonAdminForcedTakeover:       {},
}

// NormalizeEndedReason maps legacy/free-text ended_reason values to canonical codes.
func NormalizeEndedReason(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return EndedReasonUnknown
	}
	switch v {
	case "stale_session_reclaimed":
		return EndedReasonSupersededBySameOperator
	case "admin_forced_takeover":
		return EndedReasonSupersededByAdminTakeover
	}
	if _, ok := allowedEndedReasons[v]; ok {
		return v
	}
	return EndedReasonUnknown
}
