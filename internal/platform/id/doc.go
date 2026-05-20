// Package id generates internal resource identifiers.
//
// Use NewUUIDV7 for new database primary keys and other internally issued resource UUIDs.
// Do not use this package for JWT jti, HTTP request IDs, idempotency keys, or opaque secrets.
package id
