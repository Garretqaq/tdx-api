//go:build live

package protocol

import (
	"testing"
	"time"
)

func step(t *testing.T, name string, fn func() error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: panic: %v", name, r)
		}
	}()
	if err := fn(); err != nil {
		t.Errorf("%s: %v", name, err)
	}
}

func TestLiveProbe(t *testing.T) {
	var c *Conn
	for _, h := range Hosts {
		cc, err := Dial(h, 10*time.Second)
		if err == nil {
			c = cc
			t.Logf("connected %s", h)
			break
		}
		t.Logf("dial %s: %v", h, err)
	}
	if c == nil {
		t.Fatal("no host reachable")
	}
	defer c.Close()

	do := func(r *Request, err error) ([]byte, error) {
		if err != nil {
			return nil, err
		}
		return c.Do(r.Type, r.Data)
	}

	step(t, "quote", func() error {
		bs, err := do(ReqQuote([]string{"600000", "510300", "sh000001"}))
		if err != nil {
			return err
		}
		qs, err := DecodeQuotes(bs)
		if err != nil {
			return err
		}
		for _, q := range qs {
			t.Logf("quote %d/%s time=%s last=%.3f open=%.3f high=%.3f low=%.3f close=%.3f vol=%d amt=%.0f bid1=%.3f(%d) ask1=%.3f(%d)", q.Exchange, q.Code, q.ServerTime, q.Last.Yuan(), q.Open.Yuan(), q.High.Yuan(), q.Low.Yuan(), q.Close.Yuan(), q.Volume, q.Amount.Yuan(), q.Bids[0].Price.Yuan(), q.Bids[0].Volume, q.Asks[0].Price.Yuan(), q.Asks[0].Volume)
		}
		return nil
	})

	kline := func(name, code string, typ uint8, idx bool, n uint16) {
		step(t, name, func() error {
			bs, err := do(ReqKline(code, typ, 0, n))
			if err != nil {
				return err
			}
			ks, err := DecodeKlines(bs, typ, idx, true)
			if err != nil {
				return err
			}
			for _, k := range ks {
				t.Logf("%s %s o=%.3f h=%.3f l=%.3f c=%.3f last=%.3f vol=%d amt=%.0f", name, k.Time.Format("2006-01-02 15:04"), k.Open.Yuan(), k.High.Yuan(), k.Low.Yuan(), k.Close.Yuan(), k.Last.Yuan(), k.Volume, k.Amount.Yuan())
			}
			return nil
		})
	}
	kline("day", "600000", KlineDay, false, 5)
	kline("week", "600000", KlineWeek, false, 3)
	kline("1m", "600000", KlineMinute, false, 3)
	kline("etf-day", "510300", KlineDay, false, 3)
	kline("idx-day", "sh000001", KlineDay, true, 3)

	step(t, "minute", func() error {
		bs, err := do(ReqMinute("600000"))
		if err != nil {
			return err
		}
		ms, err := DecodeMinute(bs)
		if err != nil {
			return err
		}
		if len(ms) > 0 {
			t.Logf("minute n=%d first=%v last=%v", len(ms), ms[0], ms[len(ms)-1])
		}
		return nil
	})

	step(t, "history-minute", func() error {
		bs, err := do(ReqHistoryMinute("20261102", "600000"))
		if err != nil {
			return err
		}
		ms, err := DecodeHistoryMinute(bs)
		if err != nil {
			return err
		}
		if len(ms) > 0 {
			t.Logf("history-minute n=%d first=%v last=%v", len(ms), ms[0], ms[len(ms)-1])
		}
		return nil
	})

	step(t, "trade", func() error {
		bs, err := do(ReqTrade("600000", 0, 3))
		if err != nil {
			return err
		}
		ts, err := DecodeTrades(bs, time.Now().Format("20060102"), false)
		if err != nil {
			return err
		}
		for _, x := range ts {
			t.Logf("trade %s price=%.3f vol=%d side=%d", x.Time.Format("15:04:05"), x.Price.Yuan(), x.Volume, x.Side)
		}
		return nil
	})

	step(t, "etf-trade", func() error {
		bs, err := do(ReqTrade("510300", 0, 3))
		if err != nil {
			return err
		}
		ts, err := DecodeTrades(bs, time.Now().Format("20060102"), true)
		if err != nil {
			return err
		}
		for _, x := range ts {
			t.Logf("etf-trade %s price=%.3f vol=%d side=%d", x.Time.Format("15:04:05"), x.Price.Yuan(), x.Volume, x.Side)
		}
		return nil
	})

	step(t, "history-trade", func() error {
		bs, err := do(ReqHistoryTrade("20261102", "600000", 0, 3))
		if err != nil {
			return err
		}
		ts, err := DecodeHistoryTrades(bs, "20261102", false)
		if err != nil {
			return err
		}
		for _, x := range ts {
			t.Logf("history-trade %s price=%.3f vol=%d side=%d", x.Time.Format("15:04:05"), x.Price.Yuan(), x.Volume, x.Side)
		}
		return nil
	})

	step(t, "gbbq", func() error {
		bs, err := do(ReqGbbq("600000"))
		if err != nil {
			return err
		}
		xs, err := DecodeXRXDs(bs)
		if err != nil {
			return err
		}
		t.Logf("gbbq xrxd count=%d", len(xs))
		for i := len(xs) - 3; i < len(xs); i++ {
			if i >= 0 {
				x := xs[i]
				t.Logf("gbbq %s %s fenhong=%.2f peigujia=%.2f songzhuan=%.2f peigu=%.2f", x.Code, x.Time.Format("2006-01-02"), x.Fenhong, x.Peigujia, x.Songzhuangu, x.Peigu)
			}
		}
		return nil
	})

	step(t, "codes-sh", func() error {
		bs, err := do(ReqCodes(ExchangeSH, 0), nil)
		if err != nil {
			return err
		}
		cs, err := DecodeCodes(bs)
		if err != nil {
			return err
		}
		t.Logf("codes-sh n=%d", len(cs))
		for i := 0; i < 3 && i < len(cs); i++ {
			t.Logf("codes-sh %s %s decimal=%d multiple=%d", cs[i].Code, cs[i].Name, cs[i].Decimal, cs[i].Multiple)
		}
		return nil
	})
}
