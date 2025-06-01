package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"
)

var (
	caCert     *x509.Certificate
	caKey      *rsa.PrivateKey
	certCache  = make(map[string]*tls.Certificate)
	cacheMutex sync.RWMutex
)

// Initialize CA certificate and key
func init() {
	if err := generateCACertificate(); err != nil {
		panic(fmt.Sprintf("Cannot generate CA certificate: %v", err))
	}
}

// generateCACertificate generates a new self-signed CA certificate and key in memory
func generateCACertificate() error {
	// Generate CA private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("cannot generate CA private key: %v", err)
	}

	// Prepare CA certificate template
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("cannot generate CA serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:  []string{"AnyAIProxy"},
			Country:       []string{"CN"},
			Province:      []string{"Shanghai"},
			Locality:      []string{"Shanghai"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
			CommonName:    "AnyAIProxy CA",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(3650 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Create the CA certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("cannot create CA certificate: %v", err)
	}

	// Parse the created certificate
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return fmt.Errorf("cannot parse generated CA certificate: %v", err)
	}

	// Store in memory
	caCert = cert
	caKey = priv

	return nil
}

// GenerateCertificate generates a certificate for the specified domain
func GenerateCertificate(host string) (*tls.Certificate, error) {
	cacheMutex.RLock()
	if cert, ok := certCache[host]; ok {
		cacheMutex.RUnlock()
		return cert, nil
	}
	cacheMutex.RUnlock()

	// Generate new private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("cannot generate private key: %v", err)
	}

	// Prepare certificate template
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("cannot generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"MITM Proxy"},
			CommonName:   host,
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(3650 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add domain to SAN
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
	}

	// Sign with CA certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &priv.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create certificate: %v", err)
	}

	// Encode to PEM format
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	// Create TLS certificate
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("cannot create TLS certificate: %v", err)
	}

	// Cache certificate
	cacheMutex.Lock()
	certCache[host] = &tlsCert
	cacheMutex.Unlock()

	return &tlsCert, nil
}
