package biz

import "fmt"

func (uc *AlertUsecase) buildMessage(rule *AlertRule, metric *AlertMetric) string {
	switch rule.Type {
	case AlertTypeService:
		return uc.t.Get("service %s is not running", metric.Target)
	case AlertTypeProject:
		return uc.t.Get("project %s is not running", metric.Target)
	case AlertTypeContainer:
		return uc.t.Get("container %s is not running", metric.Target)
	case AlertTypeApp:
		return uc.t.Get("app %s is not running", metric.Target)
	case AlertTypeDatabase:
		return uc.t.Get("database server %s is unreachable", metric.Target)
	case AlertTypeCertExpire:
		return uc.t.Get("certificate %s expires in %s days", metric.Target, uc.formatValue(rule.Type, metric.Value))
	case AlertTypeWebsiteExpire:
		return uc.t.Get("website %s expires in %s days", metric.Target, uc.formatValue(rule.Type, metric.Value))
	}

	return uc.t.Get("%s is %s, %s threshold %s", uc.metricLabel(rule.Type, metric.Target), uc.formatValue(rule.Type, metric.Value), uc.operatorLabel(rule.Operator), uc.formatValue(rule.Type, rule.Threshold))
}

func (uc *AlertUsecase) metricLabel(typ, target string) string {
	var label string
	switch typ {
	case AlertTypeCPU:
		label = uc.t.Get("CPU usage")
	case AlertTypeMemory:
		label = uc.t.Get("memory usage")
	case AlertTypeSwap:
		label = uc.t.Get("swap usage")
	case AlertTypeLoad1:
		label = uc.t.Get("1 minute load")
	case AlertTypeLoad5:
		label = uc.t.Get("5 minutes load")
	case AlertTypeLoad15:
		label = uc.t.Get("15 minutes load")
	case AlertTypeDisk:
		label = uc.t.Get("disk usage")
	case AlertTypeDiskInode:
		label = uc.t.Get("disk inode usage")
	case AlertTypeDiskRead:
		label = uc.t.Get("disk read speed")
	case AlertTypeDiskWrite:
		label = uc.t.Get("disk write speed")
	case AlertTypeNetIn:
		label = uc.t.Get("network download speed")
	case AlertTypeNetOut:
		label = uc.t.Get("network upload speed")
	case AlertTypeWebsite5xx:
		label = uc.t.Get("website 5xx responses this hour")
	case AlertTypeWebsiteError:
		label = uc.t.Get("website error rate this hour")
	case AlertTypeService:
		label = uc.t.Get("service status")
	case AlertTypeProject:
		label = uc.t.Get("project status")
	case AlertTypeContainer:
		label = uc.t.Get("container status")
	case AlertTypeApp:
		label = uc.t.Get("app status")
	case AlertTypeDatabase:
		label = uc.t.Get("database server status")
	case AlertTypeCertExpire:
		label = uc.t.Get("certificate expiry")
	case AlertTypeWebsiteExpire:
		label = uc.t.Get("website expiry")
	default:
		label = typ
	}

	if target != "" {
		return fmt.Sprintf("%s (%s)", label, target)
	}

	return label
}

func (uc *AlertUsecase) operatorLabel(operator string) string {
	switch operator {
	case AlertOperatorGTE:
		return uc.t.Get("greater than or equal to")
	case AlertOperatorLT:
		return uc.t.Get("less than")
	case AlertOperatorLTE:
		return uc.t.Get("less than or equal to")
	default:
		return uc.t.Get("greater than")
	}
}

func (uc *AlertUsecase) formatValue(typ string, value float64) string {
	switch typ {
	case AlertTypeCPU, AlertTypeMemory, AlertTypeSwap, AlertTypeDisk, AlertTypeDiskInode, AlertTypeWebsiteError:
		return fmt.Sprintf("%.2f%%", value)
	case AlertTypeNetIn, AlertTypeNetOut, AlertTypeDiskRead, AlertTypeDiskWrite:
		return fmt.Sprintf("%.2f MB/s", value)
	case AlertTypeCertExpire, AlertTypeWebsiteExpire, AlertTypeWebsite5xx:
		return fmt.Sprintf("%.0f", value)
	}

	if uc.isStatusType(typ) {
		return uc.t.Get("not running")
	}

	return fmt.Sprintf("%.2f", value)
}

// needsSystemInfo 该规则类型是否依赖 CurrentInfo 采集的系统指标
func (uc *AlertUsecase) needsSystemInfo(typ string) bool {
	switch typ {
	case AlertTypeCPU, AlertTypeMemory, AlertTypeSwap,
		AlertTypeLoad1, AlertTypeLoad5, AlertTypeLoad15,
		AlertTypeDisk, AlertTypeDiskInode,
		AlertTypeNetIn, AlertTypeNetOut, AlertTypeDiskRead, AlertTypeDiskWrite:
		return true
	}

	return false
}

// isStatusType 状态类指标只有「运行/未运行」两种取值
func (uc *AlertUsecase) isStatusType(typ string) bool {
	switch typ {
	case AlertTypeService, AlertTypeProject, AlertTypeContainer, AlertTypeApp, AlertTypeDatabase:
		return true
	}

	return false
}

// statusValue 将运行状态转为告警取值，未运行记为 1，配合 normalizeRule 的 >=1 判定
func (uc *AlertUsecase) statusValue(running bool) float64 {
	if running {
		return 0
	}

	return 1
}

// normalizeRule 补齐规则的默认值，状态类规则固定为「不在运行」
func (uc *AlertUsecase) normalizeRule(rule *AlertRule) {
	if uc.isStatusType(rule.Type) {
		rule.Operator = AlertOperatorGTE
		rule.Threshold = 1
	}
	if rule.Duration < 1 {
		rule.Duration = 1
	}
}

func (uc *AlertUsecase) matchThreshold(rule *AlertRule, value float64) bool {
	switch rule.Operator {
	case AlertOperatorGTE:
		return value >= rule.Threshold
	case AlertOperatorLT:
		return value < rule.Threshold
	case AlertOperatorLTE:
		return value <= rule.Threshold
	default:
		return value > rule.Threshold
	}
}

func (uc *AlertUsecase) stateKey(ruleID uint, target string) string {
	return fmt.Sprintf("%d:%s", ruleID, target)
}
