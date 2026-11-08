package stalwart

import "encoding/json"

func jsonUnmarshal(raw json.RawMessage, v any) error {
	return json.Unmarshal(raw, v)
}

// listenerSpec 返回面板支持开关的 STARTTLS 监听器定义
func listenerSpec(name string) map[string]any {
	spec := map[string]any{
		"name":        name,
		"useTls":      true,
		"implicitTls": false,
	}
	switch name {
	case "submission":
		spec["protocol"] = "smtp"
		spec["bind"] = map[string]bool{"[::]:587": true}
	case "imap":
		spec["protocol"] = "imap"
		spec["bind"] = map[string]bool{"[::]:143": true}
	case "pop3":
		spec["protocol"] = "pop3"
		spec["bind"] = map[string]bool{"[::]:110": true}
	}
	return spec
}

// listenerPort 是可开关监听器对应的端口
func listenerPort(name string) uint {
	switch name {
	case "submission":
		return 587
	case "imap":
		return 143
	case "pop3":
		return 110
	}
	return 0
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
