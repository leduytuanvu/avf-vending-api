package cash

import "errors"

var (
	ErrCollectionNotFound   = errors.New("cash settlement: collection not found")
	ErrClosePayloadConflict = errors.New("cash settlement: close payload conflicts with stored close")
	ErrOpenCollectionExists = errors.New("cash settlement: machine already has an open cash collection")
	ErrInvalidCountedAmount = errors.New("cash settlement: counted amount invalid")
	ErrCurrencyMismatch     = errors.New("cash settlement: currency does not match open collection")
)
