package assortmentapp

import "errors"

// ErrBindFailed is returned when the assortment cannot be bound (e.g. org mismatch or unknown assortment).
var ErrBindFailed = errors.New("assortment: bind produced no rows")
