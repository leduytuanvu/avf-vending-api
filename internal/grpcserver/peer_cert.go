package grpcserver

import (
	"context"
	"crypto/x509"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// PeerClientCertificate returns the TLS peer leaf certificate when the RPC used TLS with a client cert.
func PeerClientCertificate(ctx context.Context) (*x509.Certificate, bool) {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil || p.AuthInfo == nil {
		return nil, false
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsInfo.State.PeerCertificates) == 0 {
		return nil, false
	}
	return tlsInfo.State.PeerCertificates[0], true
}
