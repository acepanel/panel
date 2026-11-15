package punycode

import (
	"fmt"
	"slices"

	"golang.org/x/net/idna"
)

// EncodeDomains 将 Unicode 域名列表编码为 Punycode
func EncodeDomains(domain []string) (encoded []string, err error) {
	var punycode string
	for item := range slices.Values(domain) {
		punycode, err = encodeDomain(item)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, punycode)
	}
	return encoded, nil
}

// DecodeDomain 将 Punycode 域名解码为 Unicode 域名
func DecodeDomain(punycodeDomain string) (string, error) {
	unicode, err := idna.ToUnicode(punycodeDomain)
	if err != nil {
		return "", fmt.Errorf("domain decode failed: %w", err)
	}
	return unicode, nil
}

// DecodeDomains 将 Punycode 域名列表解码为 Unicode 域名
func DecodeDomains(punycode []string) (decoded []string, err error) {
	var unicode string
	for item := range slices.Values(punycode) {
		unicode, err = DecodeDomain(item)
		if err != nil {
			return nil, err
		}
		decoded = append(decoded, unicode)
	}
	return decoded, nil
}

// encodeDomain 将 Unicode 域名编码为 Punycode
func encodeDomain(domain string) (string, error) {
	ascii, err := idna.ToASCII(domain)
	if err != nil {
		return "", fmt.Errorf("domain encode failed: %w", err)
	}
	return ascii, nil
}
