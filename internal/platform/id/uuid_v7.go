package id

import (
	"fmt"

	"github.com/google/uuid"
)

// NewUUIDV7 returns a new RFC 9562 UUID version 7 suitable for internal resource primary keys.
// It panics only if the runtime cannot produce a v7 UUID (for example an unreliable clock).
func NewUUIDV7() uuid.UUID {
	u, err := uuid.NewV7()
	if err != nil {
		panic(fmt.Sprintf("id: NewUUIDV7: %v", err))
	}
	return u
}

// NewUUIDV7String is shorthand for NewUUIDV7().String().
func NewUUIDV7String() string {
	return NewUUIDV7().String()
}
