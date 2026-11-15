package disk

import (
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestParseMdstat(t *testing.T) {
	tests := []struct {
		name   string
		mdstat string
		want   []string
	}{
		{
			name:   "没有加载任何 RAID 级别",
			mdstat: "Personalities : \nunused devices: <none>\n",
		},
		{
			name:   "加载了 RAID 级别但没有阵列",
			mdstat: "Personalities : [raid1] [raid6] [raid5] [raid4]\nunused devices: <none>\n",
		},
		{
			name: "多个阵列",
			mdstat: `Personalities : [raid1] [raid6] [raid5] [raid4]
md127 : active raid5 sdd1[3] sdc1[1] sdb1[0]
      2093056 blocks super 1.2 level 5, 512k chunk, algorithm 2 [3/3] [UUU]

md0 : active raid1 sdb2[1] sda2[0]
      1046528 blocks super 1.2 [2/2] [UU]

unused devices: <none>
`,
			want: []string{"md127", "md0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.DeepEqual(t, parseMdstat(tt.mdstat), tt.want)
		})
	}
}

func TestParseMdadmDevices(t *testing.T) {
	tests := []struct {
		name string
		rows string
		want []RAIDDevice
	}{
		{
			name: "在线盘",
			rows: "       0       8        1        0      active sync   /dev/sda1",
			want: []RAIDDevice{{Name: "/dev/sda1", Slot: "0", State: "active sync"}},
		},
		{
			name: "备盘",
			rows: "       2       8       33        -      spare   /dev/sdc1",
			want: []RAIDDevice{{Name: "/dev/sdc1", Slot: "-", State: "spare"}},
		},
		{
			name: "故障盘",
			rows: "       1       8       17        -      faulty   /dev/sdb1",
			want: []RAIDDevice{{Name: "/dev/sdb1", Slot: "-", State: "faulty"}},
		},
		{
			name: "重建中的备盘",
			rows: "       2       8       33        1      spare rebuilding   /dev/sdc1",
			want: []RAIDDevice{{Name: "/dev/sdc1", Slot: "1", State: "spare rebuilding"}},
		},
		{
			name: "RAID10 带副本组",
			rows: "       0       8        1        0      active sync set-A   /dev/sda1",
			want: []RAIDDevice{{Name: "/dev/sda1", Slot: "0", State: "active sync set-A"}},
		},
		{
			name: "已移除的槽位没有设备",
			rows: "       -       0        0        1      removed",
		},
		{
			name: "降级阵列混合各种状态",
			rows: `       0       8        1        0      active sync   /dev/sda1
       -       0        0        1      removed

       1       8       17        -      faulty   /dev/sdb1
       2       8       33        -      spare   /dev/sdc1`,
			want: []RAIDDevice{
				{Name: "/dev/sda1", Slot: "0", State: "active sync"},
				{Name: "/dev/sdb1", Slot: "-", State: "faulty"},
				{Name: "/dev/sdc1", Slot: "-", State: "spare"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arr := parseMdadm("md0", "    Number   Major   Minor   RaidDevice State\n"+tt.rows+"\n")
			check.DeepEqual(t, arr.Devices, tt.want)
		})
	}
}

func TestParseMdadm(t *testing.T) {
	arr := parseMdadm("md0", `/dev/md0:
           Version : 1.2
     Creation Time : Mon Nov  2 10:15:20 2026
        Raid Level : raid5
        Array Size : 2093056 (2044.00 MiB 2143.29 MB)
     Used Dev Size : 1046528 (1022.00 MiB 1071.64 MB)
      Raid Devices : 3
     Total Devices : 3
       Persistence : Superblock is persistent

       Update Time : Mon Nov  2 10:20:11 2026
             State : clean, degraded, recovering
    Active Devices : 2
   Working Devices : 3
    Failed Devices : 0
     Spare Devices : 1

            Layout : left-symmetric
        Chunk Size : 512K

Consistency Policy : resync

    Rebuild Status : 37% complete

              Name : host:0  (local to host host)
              UUID : 0a1b2c3d:4e5f6071:8293a4b5:c6d7e8f9
            Events : 42

    Number   Major   Minor   RaidDevice State
       0       8       17        0      active sync   /dev/sdb1
       1       8       33        1      active sync   /dev/sdc1
       3       8       49        2      spare rebuilding   /dev/sdd1
`)
	check.DeepEqual(t, arr, RAIDArray{
		Name:          "md0",
		RaidLevel:     "raid5",
		Size:          "2093056 (2044.00 MiB 2143.29 MB)",
		State:         "clean, degraded, recovering",
		StripSize:     "512K",
		ActiveDevices: 2,
		TotalDevices:  3,
		RebuildPct:    "37% complete",
		Devices: []RAIDDevice{
			{Name: "/dev/sdb1", Slot: "0", State: "active sync"},
			{Name: "/dev/sdc1", Slot: "1", State: "active sync"},
			{Name: "/dev/sdd1", Slot: "2", State: "spare rebuilding"},
		},
	})
}

func TestParseMegaRAID(t *testing.T) {
	controllers, arrays := parseMegaRAID(`{
"Controllers":[
{
	"Command Status" : {"CLI Version" : "007.1907.0000.0000", "Controller" : 0, "Status" : "Success"},
	"Response Data" : {
		"Basics" : {"Controller" : 0, "Model" : "PERC H730 Mini", "Serial Number" : "56A00XY", "FW Package Build" : "25.5.9.0001"},
		"VD LIST" : [
			{"DG/VD" : "0/0", "TYPE" : "RAID1", "State" : "Optl", "Size" : "558.375 GB"},
			{"DG/VD" : "1/1", "TYPE" : "RAID5", "State" : "Dgrd", "Size" : "1.635 TB"}
		],
		"PD LIST" : [
			{"EID:Slt" : "32:0", "State" : "Onln", "DG" : 0, "Size" : "558.375 GB", "Model" : "ST600MM0088", "SN" : "W420A1B2"},
			{"EID:Slt" : "32:2", "State" : "Onln", "DG" : 1, "Size" : "837.843 GB", "Model" : "ST900MM0168", "SN" : "W4E0C3D4"},
			{"EID:Slt" : "32:5", "State" : "UGood", "DG" : "-", "Size" : "837.843 GB", "Model" : "ST900MM0168", "SN" : "W4E0E5F6"}
		]
	}
}
]
}`)
	check.DeepEqual(t, controllers, []RAIDController{
		{Model: "PERC H730 Mini", Serial: "56A00XY", Firmware: "25.5.9.0001"},
	})
	check.DeepEqual(t, arrays, []RAIDArray{
		{
			Name: "0/0", RaidLevel: "RAID1", Size: "558.375 GB", State: "Optl",
			Devices: []RAIDDevice{{Slot: "32:0", Size: "558.375 GB", State: "Onln", Model: "ST600MM0088", Serial: "W420A1B2"}},
		},
		{
			Name: "1/1", RaidLevel: "RAID5", Size: "1.635 TB", State: "Dgrd",
			Devices: []RAIDDevice{{Slot: "32:2", Size: "837.843 GB", State: "Onln", Model: "ST900MM0168", Serial: "W4E0C3D4"}},
		},
	})

	controllers, arrays = parseMegaRAID("storcli: not json")
	check.Len(t, controllers, 0)
	check.Len(t, arrays, 0)
}

func TestParseHPSA(t *testing.T) {
	controllers, arrays := parseHPSA(`
Smart Array P440ar in Slot 0 (Embedded)
   Bus Interface: PCI
   Slot: 0
   Firmware Version: 6.60
   Total Cache Size: 2.0

   Array: A
      Interface Type: SAS
      Status: OK

      Logical Drive: 1
         Fault Tolerance: 1
         Strip Size: 256 KB

      physicaldrive 1I:1:1
         Port: 1I
         Box: 1
         Status: OK
         Size: 300 GB
         Serial Number: S0K1A1B2
         Model: HP      EG0300FBDBR

      physicaldrive 1I:1:2
         Port: 1I
         Box: 1
         Status: Predictive Failure
         Size: 300 GB
         Serial Number: S0K1C3D4
         Model: HP      EG0300FBDBR
`)
	must.Len(t, controllers, 1)
	check.Equal(t, controllers[0].Model, "Smart Array P440ar in Slot 0 (Embedded)")
	check.Equal(t, controllers[0].Firmware, "6.60")
	check.Equal(t, controllers[0].Cache, "2.0")

	must.Len(t, arrays, 1)
	check.Equal(t, arrays[0].Name, "Array: A")
	check.Equal(t, arrays[0].RaidLevel, "1")
	check.Equal(t, arrays[0].StripSize, "256 KB")
	check.DeepEqual(t, arrays[0].Devices, []RAIDDevice{
		{Name: "physicaldrive 1I:1:1", Slot: "1I", Size: "300 GB", State: "OK", Model: "HP      EG0300FBDBR", Serial: "S0K1A1B2"},
		{Name: "physicaldrive 1I:1:2", Slot: "1I", Size: "300 GB", State: "Predictive Failure", Model: "HP      EG0300FBDBR", Serial: "S0K1C3D4"},
	})
}
