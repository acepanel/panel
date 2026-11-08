package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/utils/crypt"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/notify"
)

// NotifyChannel 通知渠道
type NotifyChannel struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	Name      string          `gorm:"not null;default:''" json:"name"`
	Type      string          `gorm:"not null;default:''" json:"type"`   // smtp
	Config    json.RawMessage `gorm:"not null;default:''" json:"config"` // 渠道配置，含凭据，落库前整体加密
	Enabled   bool            `gorm:"not null;default:true" json:"enabled"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func (r *NotifyChannel) BeforeSave(tx *gorm.DB) error {
	crypter, err := crypt.NewXChacha20Poly1305([]byte(app.Key))
	if err != nil {
		return err
	}

	encrypted, err := crypter.Encrypt(r.Config)
	if err != nil {
		return err
	}
	r.Config = json.RawMessage(encrypted)

	return nil
}

func (r *NotifyChannel) AfterFind(tx *gorm.DB) error {
	crypter, err := crypt.NewXChacha20Poly1305([]byte(app.Key))
	if err != nil {
		return err
	}

	if config, err := crypter.Decrypt(string(r.Config)); err == nil {
		r.Config = config
	}

	return nil
}

// NotifyEvent 系统事件类型
type NotifyEvent string

const (
	NotifyEventCertRenew     NotifyEvent = "cert_renew"     // 证书续签失败
	NotifyEventBackup        NotifyEvent = "backup"         // 备份失败
	NotifyEventTaskFailed    NotifyEvent = "task_failed"    // 后台任务失败
	NotifyEventCronFailed    NotifyEvent = "cron_failed"    // 计划任务执行失败
	NotifyEventWebsiteExpire NotifyEvent = "website_expire" // 网站到期关停
	NotifyEventTamper        NotifyEvent = "tamper"         // 防篡改拦截
	NotifyEventHealth        NotifyEvent = "health"         // 面板健康问题
	NotifyEventLogin         NotifyEvent = "login"          // 面板登录
	NotifyEventLoginFailed   NotifyEvent = "login_failed"   // 面板登录失败过多
	NotifyEventSSHLogin      NotifyEvent = "ssh_login"      // SSH 登录
	NotifyEventSSHBruteforce NotifyEvent = "ssh_bruteforce" // SSH 爆破
)

type NotifyChannelRepo interface {
	List(page, limit uint) ([]*NotifyChannel, int64, error)
	All() ([]*NotifyChannel, error)
	Get(id uint) (*NotifyChannel, error)
	GetByIDs(ids []uint) ([]*NotifyChannel, error)
	Create(channel *NotifyChannel) error
	Update(channel *NotifyChannel) error
	Delete(id uint) error
}

// notifyMaxPending 异步事件通知的并发上限，防止高频事件堆积 goroutine
const notifyMaxPending = 32

type NotifyUsecase struct {
	repo    NotifyChannelRepo
	setting SettingRepo
	log     *slog.Logger
	t       *gotext.Locale
	pending chan struct{}
}

func NewNotifyUsecase(t *gotext.Locale, log *slog.Logger, notifyChannelRepo NotifyChannelRepo, settingRepo SettingRepo) *NotifyUsecase {
	return &NotifyUsecase{
		repo:    notifyChannelRepo,
		setting: settingRepo,
		log:     log,
		t:       t,
		pending: make(chan struct{}, notifyMaxPending),
	}
}

func (uc *NotifyUsecase) List(ctx context.Context, page, limit uint) ([]*NotifyChannel, int64, error) {
	channels, total, err := uc.repo.List(page, limit)
	if err != nil {
		return nil, 0, err
	}

	uc.log.Info(uc.t.Get("notify channel list viewed"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("page", uint64(page)), slog.Uint64("limit", uint64(limit)))

	return channels, total, nil
}

func (uc *NotifyUsecase) All() ([]*NotifyChannel, error) {
	return uc.repo.All()
}

func (uc *NotifyUsecase) Get(ctx context.Context, id uint) (*NotifyChannel, error) {
	channel, err := uc.repo.Get(id)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("notify channel viewed"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(channel.ID)), slog.String("name", channel.Name))

	return channel, nil
}

func (uc *NotifyUsecase) Create(ctx context.Context, req *request.NotifyChannelCreate) (*NotifyChannel, error) {
	// 提前构造一次，配置不合法直接拒绝入库
	if _, err := notify.New(req.Type, req.Config); err != nil {
		return nil, errors.New(uc.t.Get("invalid channel config: %v", err))
	}

	channel := &NotifyChannel{
		Name:    req.Name,
		Type:    req.Type,
		Config:  req.Config,
		Enabled: req.Enabled,
	}
	if err := uc.repo.Create(channel); err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("notify channel created"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(channel.ID)), slog.String("name", req.Name), slog.String("channel_type", req.Type), slog.Bool("enabled", req.Enabled))

	// 落库时 Config 已被加密，重新读取以返回解密后的实体
	return uc.repo.Get(channel.ID)
}

func (uc *NotifyUsecase) Update(ctx context.Context, req *request.NotifyChannelUpdate) error {
	channel, err := uc.repo.Get(req.ID)
	if err != nil {
		return err
	}
	if _, err = notify.New(req.Type, req.Config); err != nil {
		return errors.New(uc.t.Get("invalid channel config: %v", err))
	}

	channel.Name = req.Name
	channel.Type = req.Type
	channel.Config = req.Config
	channel.Enabled = req.Enabled
	if err = uc.repo.Update(channel); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("notify channel updated"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("name", req.Name), slog.String("channel_type", req.Type), slog.Bool("enabled", req.Enabled))

	return nil
}

func (uc *NotifyUsecase) Delete(ctx context.Context, id uint) error {
	channel, err := uc.repo.Get(id)
	if err != nil {
		return err
	}
	if err = uc.repo.Delete(id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("notify channel deleted"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", channel.Name))

	return nil
}

// Test 向指定渠道发送一条测试消息
func (uc *NotifyUsecase) Test(ctx context.Context, id uint) error {
	channel, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	if err = uc.dispatch(ctx, channel, uc.t.Get("[AcePanel] Test Notification"),
		NotifyBody(uc.t.Get("This is a test notification from AcePanel, receiving it means the channel is configured correctly."), [][2]string{
			{uc.t.Get("Channel"), channel.Name},
			{uc.t.Get("Time"), time.Now().Format(time.DateTime)},
		})); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("notify channel tested"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(channel.ID)), slog.String("name", channel.Name))
	return nil
}

// Send 向指定渠道列表发送通知，返回成功发送的渠道数，部分渠道失败不影响其他渠道
func (uc *NotifyUsecase) Send(ctx context.Context, channelIDs []uint, subject, body string) (int, error) {
	if len(channelIDs) == 0 {
		return 0, nil
	}

	channels, err := uc.repo.GetByIDs(channelIDs)
	if err != nil {
		return 0, err
	}

	var sent int
	var errs []error
	for _, channel := range channels {
		if !channel.Enabled {
			continue
		}
		if err = uc.dispatch(ctx, channel, subject, body); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", channel.Name, err))
			continue
		}
		sent++
	}

	return sent, errors.Join(errs...)
}

// SendEvent 发送系统事件通知，不阻塞业务流程
// 待发送数超过上限时丢弃并告知，避免慢渠道拖垮调用方
//
//nolint:contextcheck,nolintlint
func (uc *NotifyUsecase) SendEvent(event NotifyEvent, subject, body string) {
	select {
	case uc.pending <- struct{}{}:
	default:
		uc.log.Warn(uc.t.Get("event notification dropped, too many pending sends"), slog.String("event", string(event)))
		return
	}

	go func() {
		defer func() { <-uc.pending }()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := uc.SendEventSync(ctx, event, subject, body); err != nil {
			uc.log.Warn(uc.t.Get("failed to send event notification"), slog.String("event", string(event)), slog.Any("err", err))
		}
	}()
}

// EventEnabled 判断给定事件中是否有已订阅且配好渠道的
// 供调用方在采集前提前跳过，避免为没人接收的通知付出采集开销
func (uc *NotifyUsecase) EventEnabled(events ...NotifyEvent) bool {
	setting, err := uc.getSetting()
	if err != nil || len(setting.Channels) == 0 {
		return false
	}

	return lo.SomeBy(events, func(event NotifyEvent) bool {
		return slices.Contains(setting.Events, string(event))
	})
}

// SendEventSync 同步发送系统事件通知，未订阅该事件或未配置渠道时静默跳过
// 供 CLI 等短生命周期进程使用，异步发送会随进程退出丢失
func (uc *NotifyUsecase) SendEventSync(ctx context.Context, event NotifyEvent, subject, body string) error {
	setting, err := uc.getSetting()
	if err != nil || len(setting.Channels) == 0 || !slices.Contains(setting.Events, string(event)) {
		return err
	}

	_, err = uc.Send(ctx, setting.Channels, subject, body)

	return err
}

func (uc *NotifyUsecase) GetSetting(ctx context.Context) (*request.NotifySetting, error) {
	setting, err := uc.getSetting()
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("notify setting viewed"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)))

	return setting, nil
}

func (uc *NotifyUsecase) getSetting() (*request.NotifySetting, error) {
	events, err := uc.setting.GetSlice(SettingKeyNotifyEvents)
	if err != nil {
		return nil, err
	}
	channelsStr, err := uc.setting.Get(SettingKeyNotifyEventChannels)
	if err != nil {
		return nil, err
	}

	channels := make([]uint, 0)
	if channelsStr != "" {
		_ = json.Unmarshal([]byte(channelsStr), &channels)
	}

	return &request.NotifySetting{
		Events:   events,
		Channels: channels,
	}, nil
}

func (uc *NotifyUsecase) UpdateSetting(ctx context.Context, setting *request.NotifySetting) error {
	if err := uc.setting.SetSlice(SettingKeyNotifyEvents, setting.Events); err != nil {
		return err
	}

	channels, err := json.Marshal(setting.Channels)
	if err != nil {
		return err
	}

	if err = uc.setting.Set(SettingKeyNotifyEventChannels, string(channels)); err != nil {
		return err
	}

	list, _ := uc.repo.GetByIDs(setting.Channels)
	names := lo.Map(list, func(channel *NotifyChannel, _ int) string { return channel.Name })
	uc.log.Info(uc.t.Get("notify setting updated"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Any("events", setting.Events), slog.Any("channel_ids", setting.Channels), slog.Any("channels", names))

	return nil
}

func (uc *NotifyUsecase) dispatch(ctx context.Context, channel *NotifyChannel, subject, body string) error {
	notifier, err := notify.New(channel.Type, channel.Config)
	if err != nil {
		return err
	}

	return notifier.Send(ctx, &notify.Message{Subject: subject, Body: body})
}

// NotifyBody 构建通知正文，rows 为「名称，值」明细
func NotifyBody(summary string, rows [][2]string) string {
	var sb strings.Builder
	sb.WriteString(`<p>`)
	sb.WriteString(html.EscapeString(summary))
	sb.WriteString(`</p>`)

	if len(rows) > 0 {
		sb.WriteString(`<table cellpadding="6" cellspacing="0" style="border-collapse:collapse;border:1px solid #ddd">`)
		for _, row := range rows {
			sb.WriteString(`<tr><td style="border:1px solid #ddd;background:#fafafa">`)
			sb.WriteString(html.EscapeString(row[0]))
			sb.WriteString(`</td><td style="border:1px solid #ddd">`)
			sb.WriteString(html.EscapeString(row[1]))
			sb.WriteString(`</td></tr>`)
		}
		sb.WriteString(`</table>`)
	}

	return sb.String()
}
