package finance

import "errors"

var (
	// ErrDuplicateDailyClose is returned when another close already exists for the same org/date/timezone/scope.
	ErrDuplicateDailyClose = errors.New("finance: daily close already exists for this scope")
	// ErrDailyCloseNotFound is returned when a close row is missing or not tenant-visible.
	ErrDailyCloseNotFound = errors.New("finance: daily close not found")
	// ErrInvalidDailyCloseInput is returned for invalid body/query parameters.
	ErrInvalidDailyCloseInput = errors.New("finance: invalid daily close parameters")
)
