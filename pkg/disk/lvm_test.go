package disk

import (
	"testing"

	"github.com/libtnb/assert/check"
)

func TestParseLVM(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []map[string]string
	}{
		{
			name:   "没有输出",
			output: "",
		},
		{
			name:   "物理卷",
			output: "  /dev/sdb1|vg_data|<10.00g|<2.00g\n  /dev/sdc1||<10.00g|<10.00g\n",
			want: []map[string]string{
				{"field_0": "/dev/sdb1", "field_1": "vg_data", "field_2": "<10.00g", "field_3": "<2.00g"},
				{"field_0": "/dev/sdc1", "field_1": "", "field_2": "<10.00g", "field_3": "<10.00g"},
			},
		},
		{
			name:   "字段内外的空白被规整",
			output: "  lv_data | vg_data |  8.00g | /dev/vg_data/lv_data  \n\n",
			want: []map[string]string{
				{"field_0": "lv_data", "field_1": "vg_data", "field_2": "8.00g", "field_3": "/dev/vg_data/lv_data"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.DeepEqual(t, parseLVM(tt.output), tt.want)
		})
	}
}
