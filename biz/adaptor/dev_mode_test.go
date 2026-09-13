package adaptor

import "testing"

// dev 通道可以让任何人携带 mock-token 直接拿到超级管理员会话，
// 因此生产环境必须彻底关闭，这里锁定这条边界。
func TestIsProductionState(t *testing.T) {
	cases := map[string]bool{
		"prod":       true,
		"PROD":       true,
		" prod ":     true,
		"production": false, // 只认明确写法的 prod，避免误判
		"dev":        false,
		"test":       false,
		"":           false,
	}
	for state, want := range cases {
		if got := IsProductionState(state); got != want {
			t.Fatalf("IsProductionState(%q) = %v, want %v", state, got, want)
		}
	}
}
