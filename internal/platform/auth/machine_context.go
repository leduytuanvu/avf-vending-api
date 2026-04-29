package auth

import "context"

type machineAccessClaimsKey struct{}

// WithMachineAccessClaims attaches validated machine JWT claims for gRPC handlers.
func WithMachineAccessClaims(ctx context.Context, c MachineAccessClaims) context.Context {
	return context.WithValue(ctx, machineAccessClaimsKey{}, c)
}

// MachineAccessClaimsFromContext returns claims from WithMachineAccessClaims.
func MachineAccessClaimsFromContext(ctx context.Context) (MachineAccessClaims, bool) {
	v, ok := ctx.Value(machineAccessClaimsKey{}).(MachineAccessClaims)
	return v, ok
}
