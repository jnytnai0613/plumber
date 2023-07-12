/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package pki

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	replicatev1 "github.com/jnytnai0613/plumber/api/v1"
)

var (
	caTempl      = &x509.Certificate{}
	privateCaKey *rsa.PrivateKey
)

func CreateCaCrt() ([]byte, []byte, error) {
	var err error

	privateCaKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	publicCaKey := privateCaKey.Public()

	//[RFC5280]
	subjectCa := pkix.Name{
		CommonName:         "ca",
		OrganizationalUnit: []string{"Example Org Unit"},
		Organization:       []string{"Example Org"},
		Country:            []string{"JP"},
	}

	caTempl = &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               subjectCa,
		NotAfter:              time.Date(2031, 12, 31, 0, 0, 0, 0, time.UTC),
		NotBefore:             time.Now(),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	//Self Sign CA Certificate
	caCertificate, err := x509.CreateCertificate(
		rand.Reader,
		caTempl,
		caTempl,
		publicCaKey,
		privateCaKey,
	)
	if err != nil {
		return nil, nil, err
	}

	//Convert to ASN.1 PEM encoded form
	caCrt := pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: caCertificate,
		},
	)

	derCaPrivateKey := x509.MarshalPKCS1PrivateKey(privateCaKey)

	caKey := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: derCaPrivateKey,
		},
	)

	return caCrt, caKey, nil
}

func CreateSvrCrt(replicator replicatev1.Replicator) ([]byte, []byte, error) {
	privateSvrKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	publicSvrKey := privateSvrKey.Public()

	subjectSvr := pkix.Name{
		CommonName:         "server",
		OrganizationalUnit: []string{"Example Org Unit"},
		Organization:       []string{"Example Org"},
		Country:            []string{"JP"},
	}

	svrTempl := &x509.Certificate{
		SerialNumber: big.NewInt(123),
		Subject:      subjectSvr,
		NotAfter:     time.Date(2031, 12, 31, 0, 0, 0, 0, time.UTC),
		NotBefore:    time.Now(),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{*replicator.Spec.IngressSpec.Rules[0].Host},
	}

	//Server Certificate
	derSvrCertificate, err := x509.CreateCertificate(
		rand.Reader,
		svrTempl,
		caTempl,
		publicSvrKey,
		privateCaKey,
	)
	if err != nil {
		return nil, nil, err
	}

	//Convert to ASN.1 PEM encoded form
	svrCrt := pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: derSvrCertificate,
		},
	)

	derPrivateSvrKey := x509.MarshalPKCS1PrivateKey(privateSvrKey)

	svrKey := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: derPrivateSvrKey,
		},
	)

	return svrCrt, svrKey, nil
}

func CreateClientCrt() ([]byte, []byte, error) {
	privateClientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	publicClientKey := privateClientKey.Public()

	subjectClient := pkix.Name{
		CommonName:         "client",
		OrganizationalUnit: []string{"Example Org Unit"},
		Organization:       []string{"Example Org"},
		Country:            []string{"JP"},
	}

	cliTempl := &x509.Certificate{
		SerialNumber: big.NewInt(456),
		Subject:      subjectClient,
		NotAfter:     time.Date(2031, 12, 31, 0, 0, 0, 0, time.UTC),
		NotBefore:    time.Now(),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	// Client Certificate
	derClientCertificate, err := x509.CreateCertificate(
		rand.Reader,
		cliTempl,
		caTempl,
		publicClientKey,
		privateCaKey,
	)
	if err != nil {
		return nil, nil, err
	}

	// Convert to ASN.1 PEM encoded form
	cliCrt := pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: derClientCertificate,
		},
	)

	derClientPrivateKey := x509.MarshalPKCS1PrivateKey(privateClientKey)

	cliKey := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: derClientPrivateKey,
		},
	)

	return cliCrt, cliKey, nil
}
