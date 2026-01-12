package data

import (
	"context"
	"fmt"

	"github.com/moby/moby/client"
	"github.com/spf13/cast"
)

func getDockerClient(sock string) (*client.Client, error) {
	apiClient, err := client.New(client.WithHost(fmt.Sprintf("unix://%s", sock)))
	if err != nil {
		return nil, err
	}

	return apiClient, nil
}

// getOperatorID 从 context 中获取操作员ID
// 如果无法获取，返回 0（表示系统操作）
func getOperatorID(ctx context.Context) uint64 {
	if ctx == nil {
		return 0
	}
	userID := ctx.Value("user_id")
	if userID == nil {
		return 0
	}
	return cast.ToUint64(userID)
}
