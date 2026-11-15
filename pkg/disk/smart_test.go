package disk

import (
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestParseSMARTScan(t *testing.T) {
	scan := `{
  "json_format_version": [1, 0],
  "devices": [
    {"name": "/dev/sda", "info_name": "/dev/sda", "type": "scsi", "protocol": "SCSI"},
    {"name": "/dev/sdb", "info_name": "/dev/sdb [SAT]", "type": "sat", "protocol": "ATA"},
    {"name": "/dev/nvme0", "info_name": "/dev/nvme0", "type": "nvme", "protocol": "NVMe"}
  ]
}`
	models := `{
   "blockdevices": [
      {"name": "sda", "model": "PERC H730 Mini  "},
      {"name": "sdb", "model": "Samsung SSD 870 EVO 1TB"},
      {"name": "sr0", "model": null},
      {"name": "nvme0n1", "model": "SAMSUNG MZVL2512HCJQ-00B00"}
   ]
}`

	tests := []struct {
		name    string
		scan    string
		models  string
		want    []SMARTDisk
		wantErr bool
	}{
		{
			name:   "型号按设备名匹配",
			scan:   scan,
			models: models,
			want: []SMARTDisk{
				{Name: "sda", Model: "PERC H730 Mini", Type: "scsi"},
				{Name: "sdb", Model: "Samsung SSD 870 EVO 1TB", Type: "sat"},
				{Name: "nvme0", Model: "", Type: "nvme"},
			},
		},
		{
			name: "lsblk 失败时型号留空",
			scan: scan,
			want: []SMARTDisk{
				{Name: "sda", Type: "scsi"},
				{Name: "sdb", Type: "sat"},
				{Name: "nvme0", Type: "nvme"},
			},
		},
		{
			name:   "没有扫描到磁盘",
			scan:   `{"json_format_version": [1, 0], "devices": []}`,
			models: models,
			want:   []SMARTDisk{},
		},
		{
			name:    "扫描输出不是 JSON",
			scan:    "smartctl: command not found",
			models:  models,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disks, err := parseSMARTScan(tt.scan, tt.models)
			if tt.wantErr {
				must.Error(t, err)
				return
			}
			must.NoError(t, err)
			check.DeepEqual(t, disks, tt.want)
		})
	}
}
