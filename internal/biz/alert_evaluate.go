package biz

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/types"
)

// Evaluate 评估全部启用的规则，命中则记录并通知
func (uc *AlertUsecase) Evaluate(ctx context.Context) error {
	rules, err := uc.repo.AllRules()
	if err != nil {
		return err
	}

	uc.cleanup()
	uc.checkHealth(ctx)
	uc.checkSSH(ctx)

	enabled := lo.Filter(rules, func(rule *AlertRule, _ int) bool { return rule.Enabled })
	if len(enabled) == 0 {
		uc.mu.Lock()
		clear(uc.hits)
		clear(uc.silenced)
		uc.mu.Unlock()
		return nil
	}

	now := time.Now()
	var info types.CurrentInfo
	if lo.SomeBy(enabled, func(rule *AlertRule) bool { return uc.needsSystemInfo(rule.Type) }) {
		info = tools.CurrentInfo(nil, nil)
		uc.updateSnapshots(info, now)
	}

	// 同类型规则共用一次采集结果
	collected := make(map[string][]*AlertMetric)
	alive := make(map[string]struct{})
	for _, rule := range enabled {
		cacheKey := rule.Type
		if rule.Type == AlertTypeService {
			cacheKey += ":" + rule.Target
		}
		metrics, ok := collected[cacheKey]
		if !ok {
			var err error
			if metrics, err = uc.collect(ctx, rule, info); err != nil {
				uc.log.Warn(uc.t.Get("failed to collect alert metric"), slog.String("rule", rule.Name), slog.Any("err", err))
				continue
			}
			collected[cacheKey] = metrics
		}

		for _, metric := range uc.filterMetrics(rule, metrics) {
			key := uc.stateKey(rule.ID, metric.Target)
			alive[key] = struct{}{}
			uc.evaluateMetric(ctx, rule, metric, key, now)
		}
	}

	// 清理已消失的目标状态；静默记录额外要求已过期，避免某轮采集失败误清后重复告警
	uc.mu.Lock()
	maps.DeleteFunc(uc.hits, func(key string, _ uint) bool {
		_, ok := alive[key]
		return !ok
	})
	maps.DeleteFunc(uc.silenced, func(key string, until time.Time) bool {
		_, ok := alive[key]
		return !ok && now.After(until)
	})
	uc.mu.Unlock()

	return nil
}

// cleanup 按保留天数清理历史告警记录
func (uc *AlertUsecase) cleanup() {
	uc.mu.Lock()
	if time.Since(uc.cleanedAt) < 6*time.Hour {
		uc.mu.Unlock()
		return
	}
	uc.cleanedAt = time.Now()
	uc.mu.Unlock()

	days, err := uc.setting.GetInt(SettingKeyAlertLogDays, 30)
	if err != nil || days <= 0 {
		return
	}
	if err = uc.repo.ClearAlertsBefore(time.Now().AddDate(0, 0, -days)); err != nil {
		uc.log.Warn(uc.t.Get("failed to clear expired alerts"), slog.Any("err", err))
	}
}

// checkHealth 上报新出现的面板健康问题，问题恢复后重新出现会再次通知
// 同步发送并只在送达后记入去重集合，否则一次发送失败就会让持续存在的问题再也不告警
func (uc *AlertUsecase) checkHealth(ctx context.Context) {
	if !uc.notify.EventEnabled(NotifyEventHealth) {
		return
	}

	issues := app.Health.Snapshot()

	uc.mu.Lock()
	fresh := make([]app.HealthIssue, 0, len(issues))
	notified := make(map[string]struct{}, len(issues))
	for _, issue := range issues {
		if _, ok := uc.healthKeys[issue.Key]; ok {
			notified[issue.Key] = struct{}{}
			continue
		}
		fresh = append(fresh, issue)
	}
	// 已消失的问题在此被丢弃，恢复后再次出现会重新通知
	uc.healthKeys = notified
	uc.mu.Unlock()

	for _, issue := range fresh {
		if err := uc.notify.SendEventSync(ctx, NotifyEventHealth, uc.t.Get("[AcePanel] Panel Health Issue"), NotifyBody(uc.t.Get("panel reported a health issue"), [][2]string{
			{uc.t.Get("Item"), issue.Key},
			{uc.t.Get("Level"), issue.Level},
			{uc.t.Get("Detail"), issue.Message},
			{uc.t.Get("Time"), issue.Since.Format(time.DateTime)},
		})); err != nil {
			uc.log.Warn(uc.t.Get("failed to send health notification"), slog.String("item", issue.Key), slog.Any("err", err))
			continue
		}

		uc.mu.Lock()
		uc.healthKeys[issue.Key] = struct{}{}
		uc.mu.Unlock()
	}
}

// updateSnapshots 用两次采集的累计值计算速率，结果暂存于快照
func (uc *AlertUsecase) updateSnapshots(info types.CurrentInfo, now time.Time) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	nets := make(map[string]ioSnapshot, len(info.Net))
	for _, item := range info.Net {
		if item.Name == "lo" {
			continue
		}
		nets[item.Name] = uc.rateOf(uc.netSnaps[item.Name], item.BytesRecv, item.BytesSent, now)
	}
	uc.netSnaps = nets

	disks := make(map[string]ioSnapshot, len(info.DiskIO))
	for _, item := range info.DiskIO {
		disks[item.Name] = uc.rateOf(uc.diskSnaps[item.Name], item.ReadBytes, item.WriteBytes, now)
	}
	uc.diskSnaps = disks
}

// rateOf 依据上次累计值计算每秒增量，首次采集速率为 0
func (uc *AlertUsecase) rateOf(prev ioSnapshot, in, out uint64, now time.Time) ioSnapshot {
	snap := ioSnapshot{in: in, out: out, at: now}
	if prev.at.IsZero() {
		return snap
	}

	elapsed := now.Sub(prev.at).Seconds()
	if elapsed < 1 {
		elapsed = 1
	}
	if in >= prev.in {
		snap.inRate = uint64(float64(in-prev.in) / elapsed)
	}
	if out >= prev.out {
		snap.outRate = uint64(float64(out-prev.out) / elapsed)
	}

	return snap
}

// collect 采集规则类型对应的全部目标取值，不做目标过滤
func (uc *AlertUsecase) collect(ctx context.Context, rule *AlertRule, info types.CurrentInfo) ([]*AlertMetric, error) {
	switch rule.Type {
	case AlertTypeCPU:
		return []*AlertMetric{{Value: info.Percent}}, nil

	case AlertTypeMemory:
		if info.Mem == nil {
			return nil, nil
		}
		return []*AlertMetric{{Value: info.Mem.UsedPercent}}, nil

	case AlertTypeSwap:
		if info.Swap == nil || info.Swap.Total == 0 {
			return nil, nil
		}
		return []*AlertMetric{{Value: info.Swap.UsedPercent}}, nil

	case AlertTypeLoad1, AlertTypeLoad5, AlertTypeLoad15:
		if info.Load == nil {
			return nil, nil
		}
		value := info.Load.Load1
		switch rule.Type {
		case AlertTypeLoad5:
			value = info.Load.Load5
		case AlertTypeLoad15:
			value = info.Load.Load15
		}
		return []*AlertMetric{{Value: value}}, nil

	case AlertTypeDisk, AlertTypeDiskInode:
		metrics := make([]*AlertMetric, 0, len(info.DiskUsage))
		for _, usage := range info.DiskUsage {
			value := usage.UsedPercent
			if rule.Type == AlertTypeDiskInode {
				value = usage.InodesUsedPercent
			}
			metrics = append(metrics, &AlertMetric{Target: usage.Path, Value: value})
		}
		return metrics, nil

	case AlertTypeNetIn, AlertTypeNetOut, AlertTypeDiskRead, AlertTypeDiskWrite:
		return uc.rateMetrics(rule.Type), nil

	case AlertTypeWebsite5xx, AlertTypeWebsiteError:
		stats, err := uc.repo.WebsiteHourStats()
		if err != nil {
			return nil, err
		}
		metrics := make([]*AlertMetric, 0, len(stats))
		for _, item := range stats {
			if rule.Type == AlertTypeWebsite5xx {
				metrics = append(metrics, &AlertMetric{Target: item.Site, Value: float64(item.Status5xx)})
				continue
			}
			// 无请求时错误率无意义
			if item.Requests == 0 {
				continue
			}
			metrics = append(metrics, &AlertMetric{Target: item.Site, Value: float64(item.Errors) / float64(item.Requests) * 100})
		}
		return metrics, nil

	case AlertTypeService:
		if rule.Target == "" {
			return nil, nil
		}
		running, err := systemctl.Status(ctx, rule.Target)
		if err != nil {
			return nil, err
		}
		return []*AlertMetric{{Target: rule.Target, Value: uc.statusValue(running)}}, nil

	case AlertTypeProject:
		names, err := uc.repo.ProjectNames()
		if err != nil {
			return nil, err
		}
		// 项目即 systemd 单元，单元名与项目名一致，状态并发查询
		// 状态查不出来（典型是面板重启取消在途采集）就不产出指标，避免整批误判为 DOWN
		return lo.Compact(lop.Map(names, func(name string, _ int) *AlertMetric {
			running, err := systemctl.Status(ctx, name)
			if err != nil {
				return nil
			}
			return &AlertMetric{Target: name, Value: uc.statusValue(running)}
		})), nil

	case AlertTypeContainer:
		containers, err := uc.container.ListAll(ctx)
		if err != nil {
			return nil, err
		}
		metrics := make([]*AlertMetric, 0, len(containers))
		for _, item := range containers {
			metrics = append(metrics, &AlertMetric{Target: item.Name, Value: uc.statusValue(item.State == "running")})
		}
		return metrics, nil

	case AlertTypeApp:
		installed, err := uc.app.Installed()
		if err != nil {
			return nil, err
		}
		targets := lo.Filter(installed, func(item *App, _ int) bool {
			_, ok := uc.loader.Get(item.Slug)
			return ok
		})
		// 状态查询逐个访问 systemd，并发采集
		return lo.Compact(lop.Map(targets, func(item *App, _ int) *AlertMetric {
			a, _ := uc.loader.Get(item.Slug)
			status := a.Status(ctx)
			// 无 systemd 服务的应用没有运行状态
			if status == types.AppStatusNA {
				return nil
			}
			return &AlertMetric{Target: item.Slug, Value: uc.statusValue(status == types.AppStatusRunning)}
		})), nil

	case AlertTypeDatabase:
		servers, err := uc.repo.DatabaseServers()
		if err != nil {
			return nil, err
		}
		// 并发探测连通性，单台耗时上限由 pkg/db 各驱动的连接超时保证（5~10 秒）
		return lop.Map(servers, func(item *DatabaseServer, _ int) *AlertMetric {
			return &AlertMetric{Target: item.Name, Value: uc.statusValue(uc.database.CheckServer(ctx, item))}
		}), nil

	case AlertTypeCertExpire:
		return uc.repo.CertExpiry()

	case AlertTypeWebsiteExpire:
		return uc.repo.WebsiteExpiry()
	}

	return nil, fmt.Errorf("unsupported alert type: %s", rule.Type)
}

// rateMetrics 从快照读取速率指标（MB/s）
func (uc *AlertUsecase) rateMetrics(typ string) []*AlertMetric {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	snaps := uc.netSnaps
	if typ == AlertTypeDiskRead || typ == AlertTypeDiskWrite {
		snaps = uc.diskSnaps
	}

	metrics := make([]*AlertMetric, 0, len(snaps))
	for name, snap := range snaps {
		value := snap.inRate
		if typ == AlertTypeNetOut || typ == AlertTypeDiskWrite {
			value = snap.outRate
		}
		metrics = append(metrics, &AlertMetric{Target: name, Value: float64(value) / 1024 / 1024})
	}

	return metrics
}

// filterMetrics 按规则目标筛选采集结果，目标为空表示全部
func (uc *AlertUsecase) filterMetrics(rule *AlertRule, metrics []*AlertMetric) []*AlertMetric {
	if rule.Target == "" {
		return metrics
	}

	return lo.Filter(metrics, func(metric *AlertMetric, _ int) bool {
		if metric.Target == rule.Target {
			return true
		}
		// 证书目标是逗号分隔的多域名，允许用其中任一域名命中
		return rule.Type == AlertTypeCertExpire && slices.Contains(strings.Split(metric.Target, ","), rule.Target)
	})
}

func (uc *AlertUsecase) evaluateMetric(
	ctx context.Context,
	rule *AlertRule,
	metric *AlertMetric,
	key string,
	now time.Time,
) {
	// alertRetryDelay 通知发送失败后的重试间隔
	const alertRetryDelay = 5 * time.Minute

	uc.mu.Lock()
	if !uc.matchThreshold(rule, metric.Value) {
		delete(uc.hits, key)
		uc.mu.Unlock()
		return
	}

	uc.hits[key]++
	if uc.hits[key] < max(rule.Duration, 1) {
		uc.mu.Unlock()
		return
	}

	// 静默期内不重复告警
	if now.Before(uc.silenced[key]) {
		uc.mu.Unlock()
		return
	}
	silence := time.Duration(rule.Silence) * time.Minute //nolint:gosec
	uc.silenced[key] = now.Add(silence)
	uc.mu.Unlock()

	alert := &Alert{
		RuleID:   rule.ID,
		RuleName: rule.Name,
		Type:     rule.Type,
		Target:   metric.Target,
		Value:    metric.Value,
		Message:  uc.buildMessage(rule, metric),
	}

	sent, err := uc.notify.Send(ctx, rule.Channels, uc.t.Get("[AcePanel] Alert: %s", rule.Name), NotifyBody(alert.Message, [][2]string{
		{uc.t.Get("Rule"), rule.Name},
		{uc.t.Get("Metric"), uc.metricLabel(rule.Type, metric.Target)},
		{uc.t.Get("Current Value"), uc.formatValue(rule.Type, metric.Value)},
		{uc.t.Get("Threshold"), uc.formatValue(rule.Type, rule.Threshold)},
		{uc.t.Get("Time"), now.Format(time.DateTime)},
	}))
	if err != nil {
		uc.log.Warn(uc.t.Get("failed to send alert notification"), slog.String("rule", rule.Name), slog.Any("err", err))
	}
	alert.Notified = sent > 0

	// 一条都没送达时缩短静默期，让下一轮重试，避免临时故障吞掉整个静默窗口
	if len(rule.Channels) > 0 && sent == 0 {
		uc.mu.Lock()
		uc.silenced[key] = now.Add(min(alertRetryDelay, silence))
		uc.mu.Unlock()
	}

	if err = uc.repo.AddAlert(alert); err != nil {
		uc.log.Warn(uc.t.Get("failed to save alert record"), slog.String("rule", rule.Name), slog.Any("err", err))
	}
}
