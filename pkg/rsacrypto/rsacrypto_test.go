package rsacrypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"encoding/base64"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestRSA(t *testing.T) {
	privateKey, err := GenerateKey()
	must.NoError(t, err)
	check.Equal(t, privateKey.Size(), keySize/8)

	publicStr, err := PublicKeyToString(&privateKey.PublicKey)
	check.NoError(t, err)
	check.Contains(t, publicStr, "-----BEGIN PUBLIC KEY-----")

	message := []byte("AcePanel")

	ciphertext, err := rsa.EncryptOAEP(sha512.New(), rand.Reader, &privateKey.PublicKey, message, nil)
	must.NoError(t, err)

	decrypted, err := DecryptData(privateKey, base64.StdEncoding.EncodeToString(ciphertext))
	must.NoError(t, err)
	check.Equal(t, string(decrypted), string(message))
}
