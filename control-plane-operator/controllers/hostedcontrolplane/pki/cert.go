package pki

import (
	"crypto/x509"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/hypershift/support/certs"
	"github.com/openshift/hypershift/support/config"
	"github.com/openshift/hypershift/support/util"
)

var (
	X509UsageClientAuth       = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	X509UsageServerAuth       = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	X509UsageClientServerAuth = append(X509UsageClientAuth, X509UsageServerAuth...)

	X509DefaultUsage = x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	X509SignerUsage  = X509DefaultUsage | x509.KeyUsageCertSign
)

func reconcileSignedCert(secret *corev1.Secret, ca *corev1.Secret, ownerRef config.OwnerRef, cn string, org []string, extUsages []x509.ExtKeyUsage) error {
	return reconcileSignedCertWithKeys(secret, ca, ownerRef, cn, org, extUsages, corev1.TLSCertKey, corev1.TLSPrivateKeyKey, certs.CASignerCertMapKey)
}

func reconcileSignedCertWithKeys(secret, ca *corev1.Secret, ownerRef config.OwnerRef, cn string, org []string, extUsages []x509.ExtKeyUsage, crtKey, keyKey, caKey string) error {
	return reconcileSignedCertWithKeysAndAddresses(secret, ca, ownerRef, cn, org, extUsages, crtKey, keyKey, caKey, nil, nil)
}

func reconcileSignedCertWithAddresses(secret, ca *corev1.Secret, ownerRef config.OwnerRef, cn string, org []string, extUsages []x509.ExtKeyUsage, dnsNames []string, ips []string) error {
	return reconcileSignedCertWithKeysAndAddresses(secret, ca, ownerRef, cn, org, extUsages, corev1.TLSCertKey, corev1.TLSPrivateKeyKey, certs.CASignerCertMapKey, dnsNames, ips)
}

func reconcileSignedCertWithKeysAndAddresses(secret *corev1.Secret, ca *corev1.Secret, ownerRef config.OwnerRef, cn string, org []string, extUsages []x509.ExtKeyUsage, crtKey, keyKey, caKey string, dnsNames []string, ips []string) error {
	// RKC 3
	str, _ := json.Marshal(secret)
	util.Logloud("[["+secret.Name+"---secret-3]]", string(str), util.LastSecretValue)
	util.LastSecretValue = string(str)

	ownerRef.ApplyTo(secret)
	secret.Type = corev1.SecretTypeOpaque
	return certs.ReconcileSignedCert(secret, ca, cn, org, extUsages, crtKey, keyKey, caKey, dnsNames, ips)
}
