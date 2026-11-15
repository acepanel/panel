package cert

import (
	"encoding/pem"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestGenerateSelfSigned(t *testing.T) {
	certPEM, keyPEM, err := GenerateSelfSigned([]string{"haozi.dev", "192.0.2.1", "2001:db8::1"})
	must.NoError(t, err)

	block, _ := pem.Decode(certPEM)
	must.NotNil(t, block)
	check.Equal(t, block.Type, "CERTIFICATE")

	parsed, err := ParseCert(certPEM)
	must.NoError(t, err)
	check.DeepEqual(t, parsed.DNSNames, []string{"haozi.dev"})
	check.Equal(t, parsed.Subject.CommonName, "AcePanel")
	check.DeepEqual(t, SANs(parsed), []string{"haozi.dev", "192.0.2.1", "2001:db8::1"})

	keyDecoded, _ := pem.Decode(keyPEM)
	must.NotNil(t, keyDecoded)
	check.Equal(t, keyDecoded.Type, "PRIVATE KEY")
}
