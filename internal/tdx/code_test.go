package tdx

import "testing"

func TestSetCode(t *testing.T) {
	bars := []KlineBar{{Time: "2026-11-02"}, {Time: "2026-11-03"}}
	setCode(bars, "sh600000")
	for i, b := range bars {
		if b.Code != "sh600000" {
			t.Errorf("第 %d 根K线 code = %q，期望 sh600000", i, b.Code)
		}
	}
}

func TestSetCodeEmpty(t *testing.T) {
	setCode(nil, "sh600000") // 空切片不应 panic
}
