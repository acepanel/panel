package request

type SafeUpdatePingStatus struct {
	Status bool `json:"status" form:"status"`
}
