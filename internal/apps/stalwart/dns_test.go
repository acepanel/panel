package stalwart

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const sampleZone = "v1-ed25519-20261108._domainkey.acme.test. IN TXT \"v=DKIM1; k=ed25519; h=sha256; p=eBUAV77CaQc3RgSPEhT2ggPVRDmPAbPmaZVPiCa07fU=\"\n" +
	"v1-rsa-20261108._domainkey.acme.test. IN TXT (\n" +
	"    \"v=DKIM1; k=rsa; h=sha256; p=MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAvnKqa9fOSTFmO\"\n" +
	"    \"k+p17Ps4E5eS5s025m7+qxJc3wL51KtcNlnHk59FAYkVg+9o6mtef2fiE6U3GtGgEVb6A5LDSb5BnbvOTlykT+o7mtBNLAD6CS9K+VGdRaa1WqVcEaNx3lo4axovT1tNr/mE4/H8NlXQ+Exj5qrkW4p1HCgWKiwIDAQAB\"\n" +
	")\n" +
	"acme.test. IN TXT \"v=spf1 mx -all\"\n" +
	"acme.test. IN MX 10 mail.test.local.\n" +
	"_dmarc.acme.test. IN TXT \"v=DMARC1; p=reject; rua=mailto:postmaster@acme.test\"\n" +
	"_imaps._tcp.acme.test. IN SRV 0 1 993 mail.test.local.\n" +
	"mta-sts.acme.test. IN CNAME mail.test.local.\n" +
	"_mta-sts.acme.test. IN TXT \"v=STSv1; id=10693274999637916779\"\n" +
	"autoconfig.acme.test. IN CNAME mail.test.local.\n"

func TestParseZoneFile(t *testing.T) {
	records := parseZoneFile(sampleZone, "acme.test")
	assert.Len(t, records, 9)

	byID := map[string]DNSRecord{}
	for _, r := range records {
		byID[r.ID] = r
	}

	rsa := byID["TXT:v1-rsa-20261108._domainkey.acme.test"]
	assert.Equal(t, "required", rsa.Group)
	assert.Contains(t, rsa.Value, "p=MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAvnKqa9fOSTFmOk+p17Ps4E5eS5s025m7")
	assert.NotContains(t, rsa.Value, "\"")

	spf := byID["TXT:acme.test"]
	assert.Equal(t, "v=spf1 mx -all", spf.Value)
	assert.Equal(t, "required", spf.Group)

	mx := byID["MX:acme.test"]
	assert.Equal(t, "required", mx.Group)
	assert.Equal(t, uint16(10), mx.priority)
	assert.Equal(t, "mail.test.local.", mx.target)
	assert.Equal(t, "10 mail.test.local.", mx.Value)

	dmarc := byID["TXT:_dmarc.acme.test"]
	assert.Equal(t, "required", dmarc.Group)

	srv := byID["SRV:_imaps._tcp.acme.test"]
	assert.Equal(t, "optional", srv.Group)
	assert.Equal(t, uint16(993), srv.port)
	assert.Equal(t, uint16(1), srv.weight)
	assert.Equal(t, "mail.test.local.", srv.target)

	cname := byID["CNAME:mta-sts.acme.test"]
	assert.Equal(t, "optional", cname.Group)
	assert.Equal(t, "mail.test.local.", cname.target)

	sts := byID["TXT:_mta-sts.acme.test"]
	assert.Equal(t, "optional", sts.Group)
}
