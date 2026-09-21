package temporal

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/config"
)

func certificate(t *testing.T) (certFile, keyFile string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "temporal-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	der8, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	dir := t.TempDir()
	certFile = filepath.Join(dir, "tls.crt")
	keyFile = filepath.Join(dir, "tls.key")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der8}), 0o600))

	return certFile, keyFile
}

func TestTLSConfigDisabled(t *testing.T) {
	out, err := tlsConfig(config.TemporalTLS{CertFile: "/nonexistent"})
	require.NoError(t, err)
	assert.Nil(t, out, "a disabled block must not switch TLS on")
}

func TestTLSConfigLoadsKeyPairAndCA(t *testing.T) {
	certFile, keyFile := certificate(t)

	out, err := tlsConfig(config.TemporalTLS{
		Enabled:  true,
		CertFile: certFile,
		KeyFile:  keyFile,
		CAFile:   certFile,
	})
	require.NoError(t, err)

	assert.Len(t, out.Certificates, 1)
	assert.NotNil(t, out.RootCAs)
	assert.False(t, out.InsecureSkipVerify)
}

func TestTLSConfigSkipsVerification(t *testing.T) {
	out, err := tlsConfig(config.TemporalTLS{Enabled: true, InsecureSkipVerify: true})
	require.NoError(t, err)

	assert.True(t, out.InsecureSkipVerify)
}

func TestTLSConfigWithoutClientCertificate(t *testing.T) {
	out, err := tlsConfig(config.TemporalTLS{Enabled: true})
	require.NoError(t, err)

	assert.Empty(t, out.Certificates)
	assert.Nil(t, out.RootCAs, "an empty ca_file means the system trust store")
}

func TestTLSConfigRejectsUnreadableFiles(t *testing.T) {
	certFile, _ := certificate(t)

	_, err := tlsConfig(config.TemporalTLS{Enabled: true, CertFile: certFile})
	require.ErrorContains(t, err, "client key pair")

	_, err = tlsConfig(config.TemporalTLS{Enabled: true, CAFile: filepath.Join(t.TempDir(), "absent.pem")})
	require.ErrorContains(t, err, "ca file")

	empty := filepath.Join(t.TempDir(), "empty.pem")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))
	_, err = tlsConfig(config.TemporalTLS{Enabled: true, CAFile: empty})
	require.ErrorContains(t, err, "holds no certificate")
}
