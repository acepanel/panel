package data

import "testing"

func TestSelectWebsiteDataPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		root string
		want string
	}{
		{
			name: "网站目录是运行目录父目录时使用运行目录",
			path: "/opt",
			root: "/opt/zdir",
			want: "/opt/zdir",
		},
		{
			name: "网站目录与运行目录相同时保持不变",
			path: "/opt/zdir",
			root: "/opt/zdir",
			want: "/opt/zdir",
		},
		{
			name: "运行目录是网站目录父目录时使用网站目录",
			path: "/opt/zdir",
			root: "/opt",
			want: "/opt/zdir",
		},
		{
			name: "路径无父子关系时保持网站目录",
			path: "/data/www/site",
			root: "/srv/site/public",
			want: "/data/www/site",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectWebsiteDataPath(tt.path, tt.root)
			if got != tt.want {
				t.Fatalf("selectWebsiteDataPath(%q, %q) = %q, want %q", tt.path, tt.root, got, tt.want)
			}
		})
	}
}
