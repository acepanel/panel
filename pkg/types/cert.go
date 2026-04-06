package types

import "time"

type CertList struct {
	ID          uint              `json:"id"`
	AccountID   uint              `json:"account_id"`
	WebsiteID   uint              `json:"website_id"`
	DNSID       uint              `json:"dns_id"`
	Type        string            `json:"type"`
	Domains     []string          `json:"domains"`
	Alias       map[string]string `json:"alias"`
	AutoRenewal bool              `json:"auto_renewal"`
	NextRenewal time.Time         `json:"next_renewal"`
	Cert        string            `json:"cert"`
	Key         string            `json:"key"`
	CertURL     string            `json:"cert_url"`
	Script      string            `json:"script"`
	NotBefore   time.Time         `json:"not_before"`
	NotAfter    time.Time         `json:"not_after"`
	Issuer      string            `json:"issuer"`
	OCSPServer  []string          `json:"ocsp_server"`
	DNSNames    []string          `json:"dns_names"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
