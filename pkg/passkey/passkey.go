package passkey

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/acepanel/panel/v3/internal/biz"
)

// User 适配 webauthn.User 接口
type User struct {
	Inner    *biz.User
	Passkeys []*biz.UserPasskey
}

func (u *User) WebAuthnID() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(u.Inner.ID))
	return buf
}

func (u *User) WebAuthnName() string        { return u.Inner.Username }
func (u *User) WebAuthnDisplayName() string { return u.Inner.Username }
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, 0, len(u.Passkeys))
	for _, p := range u.Passkeys {
		var transports []protocol.AuthenticatorTransport
		if p.Transports != "" {
			var ts []string
			if err := json.Unmarshal([]byte(p.Transports), &ts); err == nil {
				for _, t := range ts {
					transports = append(transports, protocol.AuthenticatorTransport(t))
				}
			}
		}
		creds = append(creds, webauthn.Credential{
			ID:        p.CredentialID,
			PublicKey: p.PublicKey,
			Transport: transports,
			Flags: webauthn.CredentialFlags{
				BackupState: p.BackupState,
			},
			Authenticator: webauthn.Authenticator{
				AAGUID:    p.AAGUID,
				SignCount: p.SignCount,
			},
		})
	}
	return creds
}

// ParseUserID 从 WebAuthnID 字节还原 user ID
func ParseUserID(userHandle []byte) (uint, error) {
	if len(userHandle) != 8 {
		return 0, fmt.Errorf("invalid user handle")
	}
	return uint(binary.BigEndian.Uint64(userHandle)), nil
}

// NewWebAuthn 根据 HTTP 请求动态创建 WebAuthn 实例
func NewWebAuthn(r *http.Request) (*webauthn.WebAuthn, error) {
	host := r.Host
	hostname, _, err := net.SplitHostPort(host)
	if err != nil {
		hostname = host
	}

	scheme := "https"
	if r.TLS == nil {
		if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
			scheme = fwd
		} else {
			scheme = "http"
		}
	}

	origin := fmt.Sprintf("%s://%s", scheme, host)

	return webauthn.New(&webauthn.Config{
		RPID:          hostname,
		RPDisplayName: "AcePanel",
		RPOrigins:     []string{origin},
	})
}
