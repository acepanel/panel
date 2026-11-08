package stalwart

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 对真实 Stalwart 实例跑一遍面板用到的管理调用，需要设置
// STALWART_TEST_URL=http://127.0.0.1:18080/jmap/ STALWART_TEST_USER=... STALWART_TEST_PASS=...
func integrationClient(t *testing.T) *client {
	t.Helper()
	url := os.Getenv("STALWART_TEST_URL")
	if url == "" {
		t.Skip("STALWART_TEST_URL not set")
	}
	c := newClient(os.Getenv("STALWART_TEST_USER"), os.Getenv("STALWART_TEST_PASS"))
	c.url = url
	return c
}

func TestIntegrationObjects(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	settings, err := getOne[swSystemSettings](ctx, c, "SystemSettings", singleton)
	require.NoError(t, err)
	assert.NotEmpty(t, settings.DefaultHostname)
	assert.NotEmpty(t, settings.DefaultDomainID)

	domains, err := get[swDomain](ctx, c, "Domain", nil, domainProps)
	require.NoError(t, err)
	require.NotEmpty(t, domains)
	assert.False(t, domains[0].CreatedAt.IsZero())

	full, err := getOne[swDomain](ctx, c, "Domain", domains[0].ID)
	require.NoError(t, err)
	records := parseZoneFile(full.DNSZoneFile, full.Name)
	assert.NotEmpty(t, records)

	accounts, total, err := queryGet[swAccount](ctx, c, "Account", map[string]any{"domainId": settings.DefaultDomainID}, 0, 10, accountProps)
	require.NoError(t, err)
	assert.Positive(t, total)
	assert.NotEmpty(t, accounts)
	assert.Equal(t, "User", accounts[0].Type)

	listeners, err := get[swListener](ctx, c, "NetworkListener", nil, []string{"name", "bind", "protocol", "implicitTls"})
	require.NoError(t, err)
	assert.NotEmpty(t, listeners)

	apps, err := get[swApplication](ctx, c, "Application", nil, []string{"urlPrefix", "resourceUrl"})
	require.NoError(t, err)
	assert.NotEmpty(t, apps)

	strategy, err := getOne[swOutboundStrategy](ctx, c, "MtaOutboundStrategy", singleton)
	require.NoError(t, err)
	assert.NotEmpty(t, strategy.Route.Else)

	routes, err := get[swRoute](ctx, c, "MtaRoute", nil, []string{"name", "host", "port", "authUsername", "implicitTls", "@type"})
	require.NoError(t, err)
	assert.NotEmpty(t, routes)

	messages, _, err := queryGet[swQueuedMessage](ctx, c, "QueuedMessage", nil, 0, 10, queueProps)
	require.NoError(t, err)
	for _, m := range messages {
		assert.NotEmpty(t, m.Recipients)
		assert.False(t, m.CreatedAt.IsZero())
	}

	// 创建再删除一个临时账户，覆盖 set 的 create/update/destroy 与错误解析
	id, err := c.create(ctx, "Account", map[string]any{
		"@type": "User", "name": "paneltest", "domainId": settings.DefaultDomainID,
		"credentials": map[string]any{"0": map[string]any{"@type": "Password", "secret": "Panel-Test-Passw0rd"}},
		"roles":       map[string]any{"@type": "User"}, "quotas": quotaMap(1 << 30), "aliases": map[string]any{},
	})
	require.NoError(t, err)
	require.NoError(t, c.update(ctx, "Account", id, map[string]any{"description": "panel"}))
	_, err = c.create(ctx, "MtaRoute", map[string]any{"@type": "Relay", "host": "x"})
	assert.ErrorContains(t, err, "name")
	require.NoError(t, c.destroy(ctx, "Account", []string{id}))
}
