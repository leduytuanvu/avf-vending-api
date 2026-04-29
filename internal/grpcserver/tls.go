package grpcserver

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/avf/avf-vending-api/internal/config"
)

func machineGRPCTLSConfig(cfg *config.Config) (*tls.Config, error) {
	if cfg == nil || !cfg.GRPC.TLS.Enabled {
		return nil, nil
	}
	t := cfg.GRPC.TLS
	cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("grpc tls: load server cert: %w", err)
	}
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	caPath := strings.TrimSpace(t.ClientCAFile)
	if caPath != "" {
		b, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("grpc tls: read client CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(b) {
			return nil, errors.New("grpc tls: GRPC_TLS_CLIENT_CA_FILE contained no PEM certificates")
		}
		tlsConf.ClientCAs = pool
	}
	switch strings.ToLower(strings.TrimSpace(t.ClientAuth)) {
	case "", "no", "none":
		tlsConf.ClientAuth = tls.NoClientCert
	case "request", "verify_if_given":
		if tlsConf.ClientCAs == nil {
			return nil, errors.New("grpc tls: client auth request requires GRPC_TLS_CLIENT_CA_FILE")
		}
		tlsConf.ClientAuth = tls.VerifyClientCertIfGiven
	case "require", "require_and_verify":
		if tlsConf.ClientCAs == nil {
			return nil, errors.New("grpc tls: client auth require requires GRPC_TLS_CLIENT_CA_FILE")
		}
		tlsConf.ClientAuth = tls.RequireAndVerifyClientCert
	default:
		return nil, fmt.Errorf("grpc tls: unknown GRPC_TLS_CLIENT_AUTH")
	}
	return tlsConf, nil
}
