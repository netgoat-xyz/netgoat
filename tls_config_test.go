package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"netgoat.xyz/agent/internal/config"
	"netgoat.xyz/agent/internal/database"
	"netgoat.xyz/agent/internal/streaming"
)

func TestConfigureTLSManagerUsesPerDomainCertificate(t *testing.T) {
	certificatePEM, privateKeyPEM := selfSignedTLSPEM(t, "api.example.test")
	db, err := database.Init(":memory:")
	if err != nil {
		t.Fatalf("database.Init(): %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	snapshot := &streaming.ConfigSnapshot{
		RoutesConfigured: true,
		Routes: map[string]streaming.RouteData{
			"api.example.test": {
				Type:           "domain",
				Target:         "http://127.0.0.1:9000",
				CertificatePEM: certificatePEM,
				PrivateKeyPEM:  privateKeyPEM,
			},
		},
	}
	if err := applySnapshotToDB(db, snapshot); err != nil {
		t.Fatalf("applySnapshotToDB(): %v", err)
	}
	resolver := database.NewRouteResolver()
	if err := resolver.Reload(db); err != nil {
		t.Fatalf("Reload(): %v", err)
	}

	cfg := &config.Config{}
	cfg.SSL.Enabled = true
	manager, acmeAddress, err := configureTLSManager(cfg, resolver)
	if err != nil {
		t.Fatalf("configureTLSManager(): %v", err)
	}
	if acmeAddress != "" {
		t.Fatalf("ACME address = %q, want empty", acmeAddress)
	}
	certificate, err := manager.GetCertificate(&tls.ClientHelloInfo{ServerName: "api.example.test"})
	if err != nil || certificate == nil || len(certificate.Certificate) == 0 {
		t.Fatalf("GetCertificate(): certificate=%v err=%v", certificate != nil, err)
	}
	parsed, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate(): %v", err)
	}
	if parsed.Subject.CommonName != "api.example.test" {
		t.Fatalf("certificate common name = %q", parsed.Subject.CommonName)
	}
}

func TestConfigureTLSManagerRejectsUnsafeACMEConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.SSL.ACME.Enabled = true
	if _, _, err := configureTLSManager(cfg, nil); err == nil || !strings.Contains(err.Error(), "ssl.enabled") {
		t.Fatalf("ACME without TLS error = %v", err)
	}

	cfg.SSL.Enabled = true
	if _, _, err := configureTLSManager(cfg, nil); err == nil || !strings.Contains(err.Error(), "accept_tos") {
		t.Fatalf("ACME without explicit terms acceptance error = %v", err)
	}

	cfg.SSL.ACME.AcceptTOS = true
	cfg.SSL.ACME.Domains = []string{"api.example.test"}
	cfg.SSL.ACME.HTTPPort = ":8443"
	cfg.SSL.Port = ":8443"
	if _, _, err := configureTLSManager(cfg, nil); err == nil || !strings.Contains(err.Error(), "must differ") {
		t.Fatalf("same TLS and ACME port error = %v", err)
	}
}

func selfSignedTLSPEM(t *testing.T, commonName string) (string, string) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey(): %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		DNSNames:     []string{commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate(): %v", err)
	}
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey(): %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}))
}
