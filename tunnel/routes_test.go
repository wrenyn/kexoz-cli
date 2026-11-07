package tunnel

import "testing"

func TestPortForPicksLongestRoute(t *testing.T) {
	tun := &Tunnel{LocalPort: 3000, Routes: []Route{
		{Path: "/api", Port: 8000},
		{Path: "/api/admin/", Port: 9000},
		{Path: "/", Port: 1234},
		{Path: "/broken", Port: 0},
	}}
	cases := map[string]int{
		"/":                3000,
		"/about":           3000,
		"/api":             8000,
		"/api/users?id=3":  8000,
		"/apiary":          3000,
		"/api/admin":       9000,
		"/api/admin/stats": 9000,
		"/broken/x":        3000,
	}
	for path, want := range cases {
		if got := tun.PortFor(path); got != want {
			t.Errorf("PortFor(%q) = %d, want %d", path, got, want)
		}
	}
}
