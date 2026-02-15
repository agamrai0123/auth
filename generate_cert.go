package main

// import (
// 	"crypto/rand"
// 	"crypto/rsa"
// 	"crypto/x509"
// 	"crypto/x509/pkix"
// 	"encoding/pem"
// 	"fmt"
// 	"math/big"
// 	"net"
// 	"os"
// 	"time"
// )

// func main() {
// 	// Create certs directory
// 	os.MkdirAll("certs", 0755)

// 	// Generate private key
// 	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "Error generating private key: %v\n", err)
// 		os.Exit(1)
// 	}

// 	// Create certificate template
// 	template := x509.Certificate{
// 		SerialNumber: big.NewInt(1),
// 		Subject: pkix.Name{
// 			CommonName: "localhost",
// 		},
// 		NotBefore:             time.Now(),
// 		NotAfter:              time.Now().AddDate(1, 0, 0), // Valid for 1 year
// 		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
// 		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
// 		BasicConstraintsValid: true,
// 		DNSNames:              []string{"localhost", "127.0.0.1"},
// 		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
// 	}

// 	// Self-sign the certificate
// 	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "Error creating certificate: %v\n", err)
// 		os.Exit(1)
// 	}

// 	// Write certificate to file
// 	certFile, err := os.Create("certs/server.crt")
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "Error creating certificate file: %v\n", err)
// 		os.Exit(1)
// 	}
// 	defer certFile.Close()

// 	pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})

// 	// Write private key to file
// 	keyFile, err := os.Create("certs/server.key")
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "Error creating key file: %v\n", err)
// 		os.Exit(1)
// 	}
// 	defer keyFile.Close()

// 	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
// 	pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyBytes})

// 	fmt.Println("✓ Self-signed TLS certificates generated successfully")
// 	fmt.Println("  - Certificate: certs/server.crt")
// 	fmt.Println("  - Private Key: certs/server.key")
// }
