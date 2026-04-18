// Package mqtt implements an EMQX-compatible MQTT subscriber and topic router for device ingest.
//
// Status: active production path when cmd/mqtt-ingest runs with broker configuration (see
// LoadBrokerFromEnv / Validate) and DATABASE_URL. Messages are dispatched to DeviceIngest, which
// internal/modules/postgres.Store implements—persistence lives in Postgres, not in this package.
//
// This package is not started from cmd/api; it is a dedicated process boundary for edge traffic.
package mqtt
