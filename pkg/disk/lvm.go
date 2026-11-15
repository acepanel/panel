package disk

import (
	"context"
	"strconv"
	"strings"

	"github.com/acepanel/panel/v3/pkg/shell"
)

// LVMInfo 物理卷、卷组与逻辑卷，每行的列按查询顺序命名为 field_0、field_1...
type LVMInfo struct {
	PVs []map[string]string `json:"pvs"`
	VGs []map[string]string `json:"vgs"`
	LVs []map[string]string `json:"lvs"`
}

// LVM 查询 LVM 的物理卷、卷组与逻辑卷
func LVM(ctx context.Context) LVMInfo {
	pvs, _ := shell.Execf(ctx, "pvdisplay -C --noheadings --separator '|' -o pv_name,vg_name,pv_size,pv_free 2>/dev/null || echo ''")
	vgs, _ := shell.Execf(ctx, "vgdisplay -C --noheadings --separator '|' -o vg_name,pv_count,lv_count,vg_size,vg_free 2>/dev/null || echo ''")
	lvs, _ := shell.Execf(ctx, "lvdisplay -C --noheadings --separator '|' -o lv_name,vg_name,lv_size,lv_path 2>/dev/null || echo ''")

	return LVMInfo{
		PVs: parseLVM(pvs),
		VGs: parseLVM(vgs),
		LVs: parseLVM(lvs),
	}
}

func parseLVM(output string) []map[string]string {
	var rows []map[string]string
	for line := range strings.SplitSeq(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		row := make(map[string]string)
		for i, field := range strings.Split(line, "|") {
			row["field_"+strconv.Itoa(i)] = strings.Join(strings.Fields(field), " ")
		}
		rows = append(rows, row)
	}

	return rows
}
