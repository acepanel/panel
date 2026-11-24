package data

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/firewall/ipset"
)

// firewallSubscriptionCache 订阅列表的本地缓存，内容是合并后的地址段
type firewallSubscriptionCache struct{}

func (firewallSubscriptionCache) path(id uint) string {
	return filepath.Join(app.Root, "panel/storage/firewall", fmt.Sprintf("subscription-%d.txt", id))
}

// load 缓存不存在时是空集合
func (c firewallSubscriptionCache) load(id uint) (ipset.Set, error) {
	f, err := os.Open(c.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return ipset.Set{}, nil
	}
	if err != nil {
		return ipset.Set{}, err
	}
	defer func() { _ = f.Close() }()

	return ipset.Parse(f)
}

type firewallSubscriptionRepo struct {
	t     *gotext.Locale
	db    *gorm.DB
	cache firewallSubscriptionCache
}

func NewFirewallSubscriptionRepo(db *gorm.DB, t *gotext.Locale) biz.FirewallSubscriptionRepo {
	return &firewallSubscriptionRepo{t: t, db: db}
}

func (r *firewallSubscriptionRepo) List(page, limit uint) ([]*biz.FirewallSubscription, int64, error) {
	subs := make([]*biz.FirewallSubscription, 0)
	var total int64
	err := r.db.Model(&biz.FirewallSubscription{}).Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&subs).Error
	return subs, total, err
}

func (r *firewallSubscriptionRepo) ListEnabled() ([]*biz.FirewallSubscription, error) {
	subs := make([]*biz.FirewallSubscription, 0)
	err := r.db.Where("status = ?", true).Order("id asc").Find(&subs).Error
	return subs, err
}

func (r *firewallSubscriptionRepo) Get(id uint) (*biz.FirewallSubscription, error) {
	sub := new(biz.FirewallSubscription)
	if err := r.db.First(sub, id).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

func (r *firewallSubscriptionRepo) Create(sub *biz.FirewallSubscription) error {
	return r.db.Create(sub).Error
}

func (r *firewallSubscriptionRepo) Save(sub *biz.FirewallSubscription) error {
	return r.db.Save(sub).Error
}

func (r *firewallSubscriptionRepo) Delete(sub *biz.FirewallSubscription) error {
	if err := r.db.Delete(sub).Error; err != nil {
		return err
	}
	_ = os.Remove(r.cache.path(sub.ID))
	return nil
}

func (r *firewallSubscriptionRepo) Fetch(ctx context.Context, sub *biz.FirewallSubscription) error {
	// maxListSize 列表体积上限，几十万条的列表也就十来兆
	const maxListSize = 64 << 20

	client := resty.New().SetTimeout(time.Minute)
	defer func(client *resty.Client) { _ = client.Close() }(client)

	resp, err := client.R().SetContext(ctx).SetResponseBodyLimit(maxListSize).Get(sub.URL)
	if err != nil {
		return err
	}
	if resp.IsStatusFailure() {
		return errors.New(r.t.Get("unexpected status code: %d", resp.StatusCode()))
	}

	set, err := ipset.Parse(bytes.NewReader(resp.Bytes()))
	if err != nil {
		return err
	}
	// 一条都解析不出来多半是地址错了或返回了报错页面，不能拿它覆盖掉还能用的旧列表
	if set.Len() == 0 {
		return errors.New(r.t.Get("no valid IP address found in the list"))
	}

	var content bytes.Buffer
	if _, err = set.WriteTo(&content); err != nil {
		return err
	}
	sub.Count = uint(set.Len())

	// 内容没变就不重写，缓存的修改时间不动，过滤表也就不用重新下发
	path := r.cache.path(sub.ID)
	if old, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(old, content.Bytes()) {
		return nil
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, content.Bytes(), 0600)
}

func (r *firewallSubscriptionRepo) Cached(id uint) bool {
	_, err := os.Stat(r.cache.path(id))
	return err == nil
}
