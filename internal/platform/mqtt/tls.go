package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

func brokerURLImpliesTLS(brokerURL string) bool {
	u := strings.TrimSpace(brokerURL)
	if u == "" {
		return false
	}
	lower := strings.ToLower(u)
	return strings.HasPrefix(lower, "ssl:") ||
		strings.HasPrefix(lower, "tls:") ||
		strings.HasPrefix(lower, "wss:")
}

func (c BrokerConfig) applySecurity(opts *pahomqtt.ClientOptions) error {
	tlsCfg, err := c.buildTLSConfig()
	if err != nil {
		return err
	}
	if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}
	return nil
}

func (c BrokerConfig) buildTLSConfig() (*tls.Config, error) {
	needTLS := c.TLSEnabled ||
		strings.TrimSpace(c.CAFile) != "" ||
		strings.TrimSpace(c.CertFile) != "" ||
		strings.TrimSpace(c.KeyFile) != "" ||
		brokerURLImpliesTLS(c.BrokerURL)
	if !needTLS {
		return nil, nil
	}
	tc := &tls.Config{MinVersion: tls.VersionTLS12}
	if name := strings.TrimSpace(c.ServerName); name != "" {
		tc.ServerName = name
	}
	if c.InsecureSkipVerify {
		tc.InsecureSkipVerify = true
	}
	if ca := strings.TrimSpace(c.CAFile); ca != "" {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		pem, err := os.ReadFile(ca)
		if err != nil {
			return nil, fmt.Errorf("mqtt: read MQTT_CA_FILE: %w", err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("mqtt: MQTT_CA_FILE contains no valid PEM certificates")
		}
		tc.RootCAs = pool
	}
	if c.CertFile != "" && c.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("mqtt: load MQTT client certificate: %w", err)
		}
		tc.Certificates = []tls.Certificate{cert}
	}
	return tc, nil
}
