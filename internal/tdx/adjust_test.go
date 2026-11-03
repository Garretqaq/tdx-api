package tdx

import (
	"testing"
	"time"

	"github.com/Garretqaq/tdx-api/internal/protocol"
)

func TestWeekKey(t *testing.T) {
	cases := map[string]string{
		"2026-11-03": "2026-11-02", // 周二 → 周一
		"2026-11-02": "2026-11-02", // 周一 → 自身
		"2026-11-08": "2026-11-02", // 周日 → 本周一
		"2026-10-30": "2026-10-26", // 周五
	}
	for in, want := range cases {
		if got := weekKey(in); got != want {
			t.Errorf("weekKey(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestAggregateWeek(t *testing.T) {
	day := []KlineBar{
		{Time: "2026-11-02", Open: 10.0, High: 10.2, Low: 9.9, Close: 10.1, Last: 9.95, Vol: 100, Amt: 1000},
		{Time: "2026-11-03", Open: 10.1, High: 10.5, Low: 10.0, Close: 10.4, Last: 10.1, Vol: 200, Amt: 2000},
		{Time: "2026-11-09", Open: 10.4, High: 10.6, Low: 10.3, Close: 10.5, Last: 10.4, Vol: 300, Amt: 3000},
	}
	got := aggregate(day, PeriodWeek)
	if len(got) != 2 {
		t.Fatalf("期望 2 根周K，得到 %d", len(got))
	}
	w := got[0]
	if w.Time != "2026-11-02" {
		t.Errorf("首根周K日期 = %q", w.Time)
	}
	if w.Open != 10.0 || w.Close != 10.4 {
		t.Errorf("首根周K 开/收 = %v/%v", w.Open, w.Close)
	}
	if w.High != 10.5 || w.Low != 9.9 {
		t.Errorf("首根周K 高/低 = %v/%v", w.High, w.Low)
	}
	if w.Vol != 300 || w.Amt != 3000 {
		t.Errorf("首根周K 量/额 = %v/%v", w.Vol, w.Amt)
	}
	if w.Last != 9.95 {
		t.Errorf("首根周K 昨收 = %v，应取首个交易日的昨收", w.Last)
	}
	if got[1].Vol != 300 {
		t.Errorf("次根周K 量 = %v", got[1].Vol)
	}
}

func TestAggregateMonth(t *testing.T) {
	day := []KlineBar{
		{Time: "2026-10-30", Open: 9.0, High: 9.5, Low: 8.9, Close: 9.4, Last: 8.95, Vol: 10, Amt: 100},
		{Time: "2026-11-02", Open: 9.4, High: 10.5, Low: 9.3, Close: 10.4, Last: 9.4, Vol: 20, Amt: 200},
		{Time: "2026-11-03", Open: 10.4, High: 10.9, Low: 10.2, Close: 10.8, Last: 10.4, Vol: 30, Amt: 300},
	}
	got := aggregate(day, PeriodMonth)
	if len(got) != 2 {
		t.Fatalf("期望 2 根月K，得到 %d", len(got))
	}
	if got[0].Time != "2026-10" || got[1].Time != "2026-11" {
		t.Fatalf("月份键错误: %q %q", got[0].Time, got[1].Time)
	}
	if got[1].Open != 9.4 || got[1].Close != 10.8 || got[1].High != 10.9 || got[1].Low != 9.3 {
		t.Errorf("11 月K线不符: %+v", got[1])
	}
	if got[1].Vol != 50 || got[1].Amt != 500 {
		t.Errorf("11 月量额不符: %+v", got[1])
	}
}

// TestApplyAdjustQFQ 验证前复权：现金分红后历史价格应下调。
func TestApplyAdjustQFQ(t *testing.T) {
	c := &Client{}
	// 2026-07-16 每股派 0.42 元，10 股派 4.20 元
	events := []xrxdEvent{{
		t: time.Date(2026, 7, 16, 15, 0, 0, 0, time.Local),
		m: 1,
		c: 4.20 / 10, // 每股净现金流出
	}}

	ks := []*protocol.Kline{
		{Time: time.Date(2026, 7, 15, 15, 0, 0, 0, time.Local), Open: protocol.FromYuan(10.00), High: protocol.FromYuan(10.10), Low: protocol.FromYuan(9.90), Close: protocol.FromYuan(10.00), Last: protocol.FromYuan(10.00)},
		{Time: time.Date(2026, 7, 16, 15, 0, 0, 0, time.Local), Open: protocol.FromYuan(9.60), High: protocol.FromYuan(9.70), Low: protocol.FromYuan(9.50), Close: protocol.FromYuan(9.60), Last: protocol.FromYuan(9.58)},
	}

	got := c.buildAdjusted(ks, events, AdjustQFQ)
	// 除权日及之后保持原值
	if d := got[1].Close.Yuan() - 9.60; d > 0.001 || d < -0.001 {
		t.Errorf("除权日收盘 = %v，应保持 9.60", got[1].Close.Yuan())
	}
	// 除权日前一日：前复权价 = 10.00 - 0.42 = 9.58
	if d := got[0].Close.Yuan() - 9.58; d > 0.001 || d < -0.001 {
		t.Errorf("除权前收盘前复权 = %v，期望 9.58", got[0].Close.Yuan())
	}
}

// TestApplyAdjustHFQ 验证后复权：最早价格锚定不变，后续价格上抬。
func TestApplyAdjustHFQ(t *testing.T) {
	c := &Client{}
	events := []xrxdEvent{{
		t: time.Date(2026, 7, 16, 15, 0, 0, 0, time.Local),
		m: 1,
		c: 4.20 / 10,
	}}
	ks := []*protocol.Kline{
		{Time: time.Date(2026, 7, 15, 15, 0, 0, 0, time.Local), Open: protocol.FromYuan(10.00), High: protocol.FromYuan(10.00), Low: protocol.FromYuan(10.00), Close: protocol.FromYuan(10.00), Last: protocol.FromYuan(10.00)},
		{Time: time.Date(2026, 7, 16, 15, 0, 0, 0, time.Local), Open: protocol.FromYuan(9.60), High: protocol.FromYuan(9.60), Low: protocol.FromYuan(9.60), Close: protocol.FromYuan(9.60), Last: protocol.FromYuan(9.58)},
	}
	got := c.buildAdjusted(ks, events, AdjustHFQ)
	// 最早价格保持不变
	if d := got[0].Close.Yuan() - 10.00; d > 0.001 || d < -0.001 {
		t.Errorf("最早收盘 = %v，应保持 10.00", got[0].Close.Yuan())
	}
	// 除权日价格上抬 0.42
	if d := got[1].Close.Yuan() - 10.02; d > 0.001 || d < -0.001 {
		t.Errorf("除权日收盘后复权 = %v，期望 10.02", got[1].Close.Yuan())
	}
}

func TestApplyAdjustNone(t *testing.T) {
	c := &Client{}
	ks := []*protocol.Kline{{Time: time.Now(), Close: protocol.FromYuan(10)}}
	got := c.buildAdjusted(ks, nil, AdjustNone)
	if len(got) != 1 || got[0] != ks[0] {
		t.Fatal("none 应原样返回")
	}
}

func TestFutureExDayIgnored(t *testing.T) {
	c := &Client{}
	// ex-day 在未来，尚未除权，不应施加
	events := []xrxdEvent{{
		t: time.Date(2099, 1, 1, 15, 0, 0, 0, time.Local),
		m: 1,
		c: 1,
	}}
	ks := []*protocol.Kline{
		{Time: time.Date(2026, 7, 15, 15, 0, 0, 0, time.Local), Close: protocol.FromYuan(10)},
	}
	got := c.buildAdjusted(ks, events, AdjustQFQ)
	if d := got[0].Close.Yuan() - 10.0; d > 0.001 || d < -0.001 {
		t.Errorf("未来除权不应影响当前价格，得到 %v", got[0].Close.Yuan())
	}
}
