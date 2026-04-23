package vm

import (
	"testing"
)

func TestFormatHostTapIP(t *testing.T) {
	tests := []struct {
		idx    int
		wantIP string
	}{
		{0, "172.16.0.1"},
		{1, "172.16.1.1"},
		{5, "172.16.5.1"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := FormatHostTapIP(tt.idx)
			if got != tt.wantIP {
				t.Errorf("FormatHostTapIP(%d) = %s, want %s", tt.idx, got, tt.wantIP)
			}
		})
	}
}

func TestFormatVMIP(t *testing.T) {
	tests := []struct {
		idx    int
		wantIP string
	}{
		{0, "172.16.0.2"},
		{1, "172.16.1.2"},
		{5, "172.16.5.2"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := FormatVMIP(tt.idx)
			if got != tt.wantIP {
				t.Errorf("FormatVMIP(%d) = %s, want %s", tt.idx, got, tt.wantIP)
			}
		})
	}
}

func TestFormatSubnet(t *testing.T) {
	tests := []struct {
		idx  int
		want string
	}{
		{0, "172.16.0.0/24"},
		{1, "172.16.1.0/24"},
		{10, "172.16.10.0/24"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := FormatSubnet(tt.idx)
			if got != tt.want {
				t.Errorf("FormatSubnet(%d) = %s, want %s", tt.idx, got, tt.want)
			}
		})
	}
}

func TestFormatNFSPort(t *testing.T) {
	basePort := BaseNFSPort
	for i := 0; i < 5; i++ {
		got := FormatNFSPort(i)
		want := basePort + i
		if got != want {
			t.Errorf("FormatNFSPort(%d) = %d, want %d", i, got, want)
		}
	}
}

func TestFormatTapDev(t *testing.T) {
	for i := 0; i < 5; i++ {
		got := FormatTapDev(i)
		want := "fc-tap" + string(rune('0'+i))
		if i > 9 {
			want = "fc-tap" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		}
		if got != want {
			t.Errorf("FormatTapDev(%d) = %s, want %s", i, got, want)
		}
	}
}
