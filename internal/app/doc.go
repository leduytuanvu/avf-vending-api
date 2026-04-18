// Package app is the composition root for the modular monolith.
//
// Versioned HTTP surface ports live in subpackage api (see api.HTTPApplication).
// cmd binaries stay thin; bootstrap wires services into the HTTP server.
package app
