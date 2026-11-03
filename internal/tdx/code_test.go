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

// TestExFactors 校验单事件复权比值 = 前收盘 × m / (前收盘 − c)。
// 例：前收 10 元，10 派 4.2 元 → c=0.42、m=1，比值 = 10/9.58。
func TestExFactors(t *testing.T) {
	bars := []KlineBar{
		{Time: "2026-07-15", Close: 10},
		{Time: "2026-07-16", Close: 9.6},
		{Time: "2026-07-17", Close: 9.8},
	}
	rows := []FactorRow{
		{Date: "2026-07-01", M: 1, C: 0.1},  // 早于K线起点，无法计算
		{Date: "2026-07-16", M: 1, C: 0.42}, // 用 07-15 收盘
		{Date: "2026-07-17", M: 1, C: 20},   // 参考价非正，无法计算
		{Date: "2026-07-18", M: 2, C: 0.5},  // 用 07-17 收盘
	}
	got := exFactors(rows, bars)
	want := []float64{0, 10.0 / 9.58, 0, 2 * 9.8 / 9.3}
	for i, w := range want {
		if rows[i].ExFactor == nil {
			if w != 0 {
				t.Errorf("第 %d 条应有 ex_factor，期望 %.6f", i, w)
			}
			continue
		}
		if w == 0 {
			t.Errorf("第 %d 条不应有 ex_factor，得到 %.6f", i, *rows[i].ExFactor)
			continue
		}
		if d := *rows[i].ExFactor - w; d > 1e-6 || d < -1e-6 {
			t.Errorf("第 %d 条 ex_factor = %.6f，期望 %.6f", i, *rows[i].ExFactor, w)
		}
	}
	_ = got
}
