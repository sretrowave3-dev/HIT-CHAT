package transport

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"testing"
)

func TestEnsureDevCertificatesSANs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "campusnet-tls-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	mockLAN := []net.IP{
		net.ParseIP("192.168.1.150"),
		net.ParseIP("10.0.0.42"),
	}

	certFile, keyFile, caFile, err := EnsureDevCertificates(tempDir, mockLAN)
	if err != nil {
		t.Fatalf("EnsureDevCertificates failed: %v", err)
	}

	// Verify file existence
	for _, f := range []string{certFile, keyFile, caFile} {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Fatalf("expected file %s does not exist", f)
		}
	}

	// Load and parse server cert
	serverCertPEM, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(serverCertPEM)
	if block == nil {
		t.Fatal("failed to decode server cert PEM")
	}
	serverCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	// Load and parse CA cert
	caCertPEM, err := os.ReadFile(caFile)
	if err != nil {
		t.Fatal(err)
	}
	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil {
		t.Fatal("failed to decode CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	// Verify server cert is verified by CA
	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	opts := x509.VerifyOptions{
		Roots:   roots,
		DNSName: "localhost",
	}
	if _, err := serverCert.Verify(opts); err != nil {
		t.Fatalf("server certificate verification failed against generated CA: %v", err)
	}

	// Verify SANs
	found192 := false
	found10 := false
	for _, ip := range serverCert.IPAddresses {
		if ip.Equal(net.ParseIP("192.168.1.150")) {
			found192 = true
		}
		if ip.Equal(net.ParseIP("10.0.0.42")) {
			found10 = true
		}
	}

	if !found192 || !found10 {
		t.Errorf("expected LAN IPs in SANs, got: %v", serverCert.IPAddresses)
	}

	// Calling EnsureDevCertificates a second time must reuse the existing CA
	_, _, _, err = EnsureDevCertificates(tempDir, mockLAN)
	if err != nil {
		t.Fatalf("re-running EnsureDevCertificates failed: %v", err)
	}
}

func TestGetLocalIPv4Addresses(t *testing.T) {
	ips, err := GetLocalIPv4Addresses()
	if err != nil {
		t.Fatalf("GetLocalIPv4Addresses error: %v", err)
	}
	t.Logf("Detected %d local IPv4 interfaces: %v", len(ips), ips)
	for _, ip := range ips {
		if ip.IsLoopback() {
			t.Errorf("Loopback address %v returned in LAN discovery", ip)
		}
		if ip.To4() == nil {
			t.Errorf("Non-IPv4 address %v returned in IPv4 discovery", ip)
		}
	}
	// Verify preferred outbound IP is first if present
	pref, _ := GetPreferredOutboundIP()
	if pref != nil && len(ips) > 0 && !IsVirtualOrHostOnlyIP(pref) {
		if !ips[0].Equal(pref) {
			t.Errorf("Expected primary routable IP %v at index 0, got %v", pref, ips[0])
		}
	}
}
