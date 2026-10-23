package transport

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
)

// IsVirtualOrHostOnlyIP returns true if the IP belongs to a known host-only virtual network (e.g. 192.168.56.x).
func IsVirtualOrHostOnlyIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	// VirtualBox Host-Only subnet: 192.168.56.0/24
	if v4[0] == 192 && v4[1] == 168 && v4[2] == 56 {
		return true
	}
	return false
}

// isVirtualOrHostOnlyInterface returns true if the interface name or IP indicates a virtual/host-only adapter.
func isVirtualOrHostOnlyInterface(name string, ip net.IP) bool {
	if IsVirtualOrHostOnlyIP(ip) {
		return true
	}
	lower := strings.ToLower(name)
	virtualKeywords := []string{
		"virtual", "vbox", "vmware", "vmnet", "hyper-v", "vethernet",
		"wsl", "docker", "container", "tailscale", "zerotier", "wireguard",
		"tap", "tun", "pseudo",
	}
	for _, kw := range virtualKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// GetPreferredOutboundIP queries the OS routing table to determine which local IPv4 address
// is utilized for active outbound LAN/Internet traffic. Does not send network packets.
func GetPreferredOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		conn, err = net.Dial("udp", "1.1.1.1:80")
		if err != nil {
			return nil, err
		}
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.To4(), nil
}

// GetLocalIPv4Addresses discovers all non-loopback, active unicast IPv4 addresses,
// prioritizing the primary routable LAN adapter first and filtering virtual host-only adapters.
func GetLocalIPv4Addresses() ([]net.IP, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("listing network interfaces: %w", err)
	}

	preferredIP, _ := GetPreferredOutboundIP()

	var physicalIPs []net.IP
	var virtualIPs []net.IP

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.To4() == nil || ip.IsLoopback() {
				continue
			}
			ipv4 := ip.To4()
			if isVirtualOrHostOnlyInterface(iface.Name, ipv4) {
				virtualIPs = append(virtualIPs, ipv4)
			} else {
				physicalIPs = append(physicalIPs, ipv4)
			}
		}
	}

	var result []net.IP
	if preferredIP != nil && !IsVirtualOrHostOnlyIP(preferredIP) {
		result = append(result, preferredIP)
	}

	appendUnique := func(list []net.IP) {
		for _, ip := range list {
			duplicate := false
			for _, r := range result {
				if r.Equal(ip) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				result = append(result, ip)
			}
		}
	}

	appendUnique(physicalIPs)
	appendUnique(virtualIPs)

	return result, nil
}

// EnsureDevCertificates guarantees an isolated Root CA and Leaf Server Certificate exist
// with Subject Alternative Names (SANs) for all LAN addresses and localhost.
func EnsureDevCertificates(certDir string, lanIPs []net.IP) (certFile, keyFile, caFile string, err error) {
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return "", "", "", fmt.Errorf("creating cert directory: %w", err)
	}

	caCertPath := filepath.Join(certDir, "ca.crt")
	caKeyPath := filepath.Join(certDir, "ca.key")
	srvCertPath := filepath.Join(certDir, "server.crt")
	srvKeyPath := filepath.Join(certDir, "server.key")

	var caPrivKey *ecdsa.PrivateKey
	var caCert *x509.Certificate

	// 1. Load or Generate Root CA
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		caPrivKey, caCert, err = generateRootCA(caCertPath, caKeyPath)
		if err != nil {
			return "", "", "", fmt.Errorf("generating root ca: %w", err)
		}
	} else {
		caPrivKey, caCert, err = loadRootCA(caCertPath, caKeyPath)
		if err != nil {
			caPrivKey, caCert, err = generateRootCA(caCertPath, caKeyPath)
			if err != nil {
				return "", "", "", fmt.Errorf("regenerating root ca: %w", err)
			}
		}
	}

	// 2. Generate Leaf Certificate valid for localhost and detected LAN IPs
	err = generateServerCertificate(caPrivKey, caCert, srvCertPath, srvKeyPath, lanIPs)
	if err != nil {
		return "", "", "", fmt.Errorf("generating server certificate: %w", err)
	}

	return srvCertPath, srvKeyPath, caCertPath, nil
}

func generateRootCA(certPath, keyPath string) (*ecdsa.PrivateKey, *x509.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"CampusNet Security Foundation"},
			CommonName:   "CampusNet Local Development Root CA",
			Country:      []string{"IN"},
			Province:     []string{"Campus"},
			Locality:     []string{"Localhost"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(3650 * 24 * time.Hour), // 10 years
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		MaxPathLenZero:        true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	// Save CA Key (0600)
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, nil, err
	}
	_ = pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	_ = keyOut.Close()

	// Save CA Cert (0644)
	certOut, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, nil, err
	}
	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	_ = certOut.Close()

	return priv, template, nil
}

func loadRootCA(certPath, keyPath string) (*ecdsa.PrivateKey, *x509.Certificate, error) {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode ca key pem")
	}
	priv, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode ca cert pem")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	return priv, cert, nil
}

func generateServerCertificate(caPriv *ecdsa.PrivateKey, caCert *x509.Certificate, certPath, keyPath string, lanIPs []net.IP) error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	dnsNames := []string{"localhost", "campusnet.local", "*.campusnet.local"}
	ipList := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	for _, ip := range lanIPs {
		ipList = append(ipList, ip)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"CampusNet Secure Node"},
			CommonName:   "CampusNet LAN Gateway",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ipList,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, caCert, &priv.PublicKey, caPriv)
	if err != nil {
		return err
	}

	// Save Server Key (0600)
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_ = pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	_ = keyOut.Close()

	// Save Server Cert (0644)
	certOut, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	_ = certOut.Close()

	return nil
}

// PrintLANBanner prints the terminal access banner and QR code for mobile connection.
func PrintLANBanner(host string, port int, lanIPs []net.IP, isTLS bool, caPath string) {
	scheme := "http"
	wsScheme := "ws"
	if isTLS {
		scheme = "https"
		wsScheme = "wss"
	}

	fmt.Println("==================================================================")
	fmt.Printf(" [CAMPUSNET ACCESS GATEWAY] Server active on port :%d\n", port)
	fmt.Println("==================================================================")
	fmt.Printf(" -> Localhost Web:      %s://localhost:%d\n", scheme, port)
	fmt.Printf(" -> Localhost Stream:   %s://localhost:%d/ws/v1/stream\n", wsScheme, port)

	primaryURL := fmt.Sprintf("%s://localhost:%d", scheme, port)
	if len(lanIPs) > 0 {
		fmt.Println("------------------------------------------------------------------")
		fmt.Println(" LAN / MOBILE ACCESS (Connected to Wi-Fi / Hotspot):")
		for i, ip := range lanIPs {
			lanURL := fmt.Sprintf("%s://%s:%d", scheme, ip.String(), port)
			tag := ""
			if IsVirtualOrHostOnlyIP(ip) {
				tag = " [Virtual/Host-Only adapter - not reachable from Wi-Fi]"
			} else if i == 0 {
				tag = " [PRIMARY ROUTABLE LAN WI-FI]"
			}
			fmt.Printf(" [%d] %s%s\n", i+1, lanURL, tag)
			if i == 0 {
				primaryURL = lanURL
			}
		}
	}

	if isTLS {
		fmt.Println("------------------------------------------------------------------")
		if caPath != "" {
			fmt.Printf(" LOCAL ROOT CA: %s\n", caPath)
			fmt.Printf(" -> Direct CA Download: %s/ca.crt\n", primaryURL)
		}
		fmt.Println(" * Note for Mobile Browsers (Chrome / Safari):")
		fmt.Println("   1. Open the QR link on your phone/tablet.")
		fmt.Println("   2. When browser shows 'Connection is not private':")
		fmt.Println("      - Android Chrome: Tap 'Advanced' -> 'Proceed to [IP] (unsafe)'.")
		fmt.Println("      - iOS Safari: Tap 'Show Details' -> 'visit this website'.")
		fmt.Println("   3. An HTTP redirect listener is also active on port :8081.")
	}

	fmt.Println("==================================================================")
	fmt.Printf(" SCAN QR CODE TO OPEN ON MOBILE / TABLET:\n (Target: %s)\n", primaryURL)
	fmt.Println()

	// Render Terminal ASCII QR code for the primary routable URL
	if qr, err := qrcode.New(primaryURL, qrcode.Medium); err == nil {
		fmt.Print(qr.ToSmallString(false))
	} else {
		fmt.Printf(" [QR generation failed: %v]\n", err)
	}

	fmt.Println()
	fmt.Println("==================================================================")
}
