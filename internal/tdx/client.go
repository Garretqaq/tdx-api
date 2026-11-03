package tdx

import (
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/Garretqaq/tdx-api/internal/protocol"
)

// pool 简单的连接池，用 channel 实现。
type pool struct {
	ch      chan *protocol.Conn
	dial    func() (*protocol.Conn, error)
	timeout time.Duration
}

func newPool(hosts []string, size int, timeout time.Duration) (*pool, error) {
	dialFn := func() (*protocol.Conn, error) {
		var lastErr error
		for _, h := range hosts {
			c, err := protocol.Dial(h, timeout)
			if err == nil {
				return c, nil
			}
			lastErr = err
		}
		return nil, fmt.Errorf("所有服务器均不可达: %w", lastErr)
	}

	p := &pool{
		ch:      make(chan *protocol.Conn, size),
		dial:    dialFn,
		timeout: timeout,
	}
	for i := 0; i < size; i++ {
		c, err := dialFn()
		if err != nil {
			return nil, err
		}
		p.ch <- c
	}
	return p, nil
}

// do 从池取连接，执行 fn。若连接已损坏（空闲超时被服务器关闭等），
// 重建连接后立刻重试一次，避免调用方感知到 broken pipe。
func (p *pool) do(fn func(c *protocol.Conn) error) error {
	c := <-p.ch
	if c.Closed() {
		var err error
		c, err = p.dial()
		if err != nil {
			p.ch <- c
			return err
		}
	}
	err := fn(c)
	if c.Closed() {
		// 连接已损坏，重建并重试一次
		nc, nerr := p.dial()
		if nerr != nil {
			p.ch <- nc // 放回新建的（下次还会重建）
			return err // 返回原始错误
		}
		err = fn(nc)
		if nc.Closed() {
			// 重试也失败，再建一条放回池，保持池容量
			if rc, rerr := p.dial(); rerr == nil {
				p.ch <- rc
			} else {
				p.ch <- nc
			}
			return err
		}
		p.ch <- nc
		return err
	}
	p.ch <- c
	return err
}

// ---- 代码缓存 ----

type codeEntry struct {
	Code    string // 带前缀，如 sh600000
	Name    string
	Market  string
	Type    string // stock / etf / index / ""
	Decimal int8
}

// ---- 复权缓存 ----

type xrxdCache struct {
	events []xrxdEvent // 按时间升序
	expire time.Time
}

type xrxdEvent struct {
	t    time.Time
	m, c float64 // 仿射系数（复权模型）
}

// ---- Client ----

// Client 封装连接池与缓存，实现 Provider 接口。
type Client struct {
	pool *pool

	codesMu sync.RWMutex
	codes   []codeEntry
	codeMap map[string]*codeEntry // full code → entry

	xrxdMu  sync.Mutex
	xrxdMap map[string]xrxdCache

	msgID uint32
}

// NewClient 建立连接池。代码表异步加载，不阻塞启动；加载期间价格缩放按代码前缀推断。
func NewClient(poolSize int, timeout time.Duration) (*Client, error) {
	p, err := newPool(protocol.Hosts, poolSize, timeout)
	if err != nil {
		return nil, fmt.Errorf("连接通达信服务器失败: %w", err)
	}
	c := &Client{
		pool:    p,
		xrxdMap: make(map[string]xrxdCache),
	}
	go func() {
		if err := c.loadCodes(); err != nil {
			log.Printf("加载代码表失败（不影响行情查询，/codes 将返回空）: %v", err)
			return
		}
		c.codesMu.RLock()
		n := len(c.codes)
		c.codesMu.RUnlock()
		log.Printf("代码表加载完成，共 %d 条", n)
	}()
	return c, nil
}

// ---- 代码表加载 ----

func (c *Client) loadCodes() error {
	var all []codeEntry

	// 北交所（market=2）不支持标准代码表协议，请求会挂起直到超时，这里只加载沪深。
	for _, ex := range []uint8{protocol.ExchangeSH, protocol.ExchangeSZ} {
		for start := uint16(0); ; start += 1000 {
			var codes []*protocol.Code
			err := c.pool.do(func(conn *protocol.Conn) error {
				req := protocol.ReqCodes(ex, start)
				bs, e := conn.Do(req.Type, req.Data)
				if e != nil {
					return e
				}
				var de error
				codes, de = protocol.DecodeCodes(bs)
				return de
			})
			if err != nil {
				return err
			}
			for _, v := range codes {
				full := protocol.ExchangeName(ex) + v.Code
				kind := protocol.Kind(full)
				if kind == "" {
					continue
				}
				all = append(all, codeEntry{
					Code:    full,
					Name:    v.Name,
					Market:  protocol.ExchangeName(ex),
					Type:    kind,
					Decimal: v.Decimal,
				})
			}
			if len(codes) < 1000 {
				break
			}
		}
	}

	m := make(map[string]*codeEntry, len(all))
	for i := range all {
		m[all[i].Code] = &all[i]
	}

	c.codesMu.Lock()
	c.codes = all
	c.codeMap = m
	c.codesMu.Unlock()
	return nil
}

// ---- Quotes ----

func (c *Client) Quotes(codes []string) ([]Quote, error) {
	if len(codes) == 0 {
		return []Quote{}, nil
	}
	var qs []*protocol.Quote
	err := c.pool.do(func(conn *protocol.Conn) error {
		req, e := protocol.ReqQuote(codes)
		if e != nil {
			return e
		}
		bs, e := conn.Do(req.Type, req.Data)
		if e != nil {
			return e
		}
		qs, e = protocol.DecodeQuotes(bs)
		return e
	})
	if err != nil {
		return nil, err
	}

	out := make([]Quote, 0, len(qs))
	for _, q := range qs {
		full := protocol.ExchangeName(q.Exchange) + q.Code
		scale := c.decimalScale(full)
		isIndex := protocol.Kind(full) == "index"
		out = append(out, Quote{
			Code:       full,
			Name:       c.nameOf(full),
			Last:       round(q.Last.Yuan()*scale, 3),
			Open:       round(q.Open.Yuan()*scale, 3),
			High:       round(q.High.Yuan()*scale, 3),
			Low:        round(q.Low.Yuan()*scale, 3),
			Close:      round(q.Close.Yuan()*scale, 3),
			Volume:     q.Volume,
			Amount:     q.Amount.Yuan(),
			Intuition:  int(q.Intuition),
			InsideDish: int(q.InsideDish),
			OuterDisc:  int(q.OuterDisc),
			Bids:       c.toLevels(q.Bids[:], scale, isIndex),
			Asks:       c.toLevels(q.Asks[:], scale, isIndex),
		})
	}
	return out, nil
}

// toLevels 转换盘口；指数没有真实盘口，返回空切片。
func (c *Client) toLevels(ls []protocol.QuoteLevel, scale float64, isIndex bool) []Level {
	if isIndex {
		return []Level{}
	}
	out := make([]Level, len(ls))
	for i, v := range ls {
		out[i] = Level{Price: round(v.Price.Yuan()*scale, 3), Volume: v.Volume}
	}
	return out
}

func (c *Client) getEntry(full string) *codeEntry {
	c.codesMu.RLock()
	defer c.codesMu.RUnlock()
	return c.codeMap[full]
}

// nameOf 返回证券名称；代码表尚未加载时为空。
func (c *Client) nameOf(full string) string {
	if e := c.getEntry(full); e != nil {
		return e.Name
	}
	return ""
}

// decimalScale 返回行情价格缩放系数。
// 代码表已加载时用 Decimal（2 为标准，3 需再除以 10）；
// 未加载时按代码前缀推断，ETF 除以 10。
func (c *Client) decimalScale(full string) float64 {
	if e := c.getEntry(full); e != nil {
		return math.Pow10(int(2 - e.Decimal))
	}
	if protocol.IsETFCode(full) {
		return 0.1
	}
	return 1
}

// ---- Klines ----

func (c *Client) Klines(code string, period Period, adjust Adjust, count int) ([]KlineBar, error) {
	if adjust != AdjustNone && (period == Period1Min || period == Period5Min ||
		period == Period15Min || period == Period30Min || period == Period60Min) {
		return nil, errors.New("分钟K线不支持复权")
	}

	full, err := protocol.FullCode(code)
	if err != nil {
		return nil, err
	}
	isIndex := protocol.Kind(full) == "index"

	// 周/月前复权：先取复权日K再合成
	if adjust != AdjustNone && (period == PeriodWeek || period == PeriodMonth) {
		dayBars, err := c.fetchKlines(full, PeriodDay, adjust, 0, isIndex)
		if err != nil {
			return nil, err
		}
		return tail(aggregate(dayBars, period), count), nil
	}

	// 后复权以全部历史的最早交易日为锚点，必须拉全量后再截取最近 count 条
	if adjust == AdjustHFQ && count > 0 {
		bars, err := c.fetchKlines(full, period, adjust, 0, isIndex)
		if err != nil {
			return nil, err
		}
		return tail(bars, count), nil
	}

	return c.fetchKlines(full, period, adjust, count, isIndex)
}

func (c *Client) fetchKlines(full string, period Period, adjust Adjust, count int, isIndex bool) ([]KlineBar, error) {
	t := klineType(period)
	normalizeVol := (t != protocol.KlineDay && t != protocol.KlineWeek && t != protocol.KlineMonth)

	var ks []*protocol.Kline
	if count > 0 {
		need := count
		for start := 0; len(ks) < need; {
			page := min(800, need-len(ks))
			var chunk []*protocol.Kline
			err := c.pool.do(func(conn *protocol.Conn) error {
				req, e := protocol.ReqKline(full, t, uint16(start), uint16(page))
				if e != nil {
					return e
				}
				bs, e := conn.Do(req.Type, req.Data)
				if e != nil {
					return e
				}
				chunk, e = protocol.DecodeKlines(bs, t, isIndex, normalizeVol)
				return e
			})
			if err != nil {
				return nil, err
			}
			ks = append(chunk, ks...) // 最新数据在前，要拼到尾部
			if len(chunk) < page {
				break
			}
			start += page
		}
		// 保留最新 count 条（start=0 是最新，依次向前）
		if len(ks) > need {
			ks = ks[len(ks)-need:]
		}
	} else {
		// 全量：从 start=0 开始拉，把 chunk 逐批拼到最前
		for start := 0; ; start += 800 {
			var chunk []*protocol.Kline
			err := c.pool.do(func(conn *protocol.Conn) error {
				req, e := protocol.ReqKline(full, t, uint16(start), 800)
				if e != nil {
					return e
				}
				bs, e := conn.Do(req.Type, req.Data)
				if e != nil {
					return e
				}
				chunk, e = protocol.DecodeKlines(bs, t, isIndex, normalizeVol)
				return e
			})
			if err != nil {
				return nil, err
			}
			ks = append(chunk, ks...)
			if len(chunk) < 800 {
				break
			}
		}
	}

	if adjust != AdjustNone && period == PeriodDay {
		ks = c.applyAdjust(full, ks, adjust)
	}

	out := make([]KlineBar, len(ks))
	for i, k := range ks {
		out[i] = KlineBar{
			Time:  fmtBarTime(k.Time, period),
			Open:  k.Open.Yuan(),
			High:  k.High.Yuan(),
			Low:   k.Low.Yuan(),
			Close: k.Close.Yuan(),
			Last:  k.Last.Yuan(),
			Vol:   k.Volume,
			Amt:   k.Amount.Yuan(),
		}
	}
	return out, nil
}

func klineType(p Period) uint8 {
	switch p {
	case Period1Min:
		return protocol.KlineMinute
	case Period5Min:
		return protocol.Kline5Minute
	case Period15Min:
		return protocol.Kline15Minute
	case Period30Min:
		return protocol.Kline30Minute
	case Period60Min:
		return protocol.Kline60Minute
	case PeriodWeek:
		return protocol.KlineWeek
	case PeriodMonth:
		return protocol.KlineMonth
	default:
		return protocol.KlineDay
	}
}

func fmtBarTime(t time.Time, p Period) string {
	if p == PeriodWeek || p == PeriodMonth || p == PeriodDay {
		return t.Format("2006-01-02")
	}
	return t.Format("2006-01-02 15:04")
}

// ---- 复权 ----

func (c *Client) applyAdjust(full string, ks []*protocol.Kline, adj Adjust) []*protocol.Kline {
	return c.buildAdjusted(ks, c.getXRXD(full), adj)
}

// buildAdjusted 用给定的除权除息事件对日K做复权，纯计算、无 IO，便于单测。
func (c *Client) buildAdjusted(ks []*protocol.Kline, events []xrxdEvent, adj Adjust) []*protocol.Kline {
	if adj == AdjustNone || len(ks) == 0 || len(events) == 0 {
		return ks
	}
	latest := ks[len(ks)-1].Time
	// 过滤掉未来 ex-day（已公告未除权）
	evs := make([]xrxdEvent, 0, len(events))
	for _, e := range events {
		if !e.t.After(latest) {
			evs = append(evs, e)
		}
	}
	if len(evs) == 0 {
		return ks
	}

	// QFQ 因子：自最新向过去，复合所有 ex-day > 当前交易日的事件
	factors := make([][2]float64, len(ks)) // [a, b]
	a, b := 1.0, 0.0
	ei := len(evs) - 1
	for i := len(ks) - 1; i >= 0; i-- {
		for ei >= 0 && evs[ei].t.After(ks[i].Time) {
			a = a / evs[ei].m
			b = b - a*evs[ei].c
			ei--
		}
		factors[i] = [2]float64{a, b}
	}

	a0, b0 := factors[0][0], factors[0][1]
	out := make([]*protocol.Kline, len(ks))
	for i, k := range ks {
		nk := *k
		fa, fb := factors[i][0], factors[i][1]
		if adj == AdjustHFQ && a0 != 0 {
			fa, fb = fa/a0, (fb-b0)/a0
		}
		price := func(p protocol.Price) protocol.Price {
			return protocol.FromYuan(round2(fa*p.Yuan() + fb))
		}
		if k.Last != 0 {
			nk.Last = price(k.Last)
		}
		nk.Open = price(k.Open)
		nk.High = price(k.High)
		nk.Low = price(k.Low)
		nk.Close = price(k.Close)
		out[i] = &nk
	}
	return out
}

func round(v float64, n int) float64 {
	p := math.Pow10(n)
	return math.Round(v*p) / p
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func (c *Client) getXRXD(full string) []xrxdEvent {
	c.xrxdMu.Lock()
	if cache, ok := c.xrxdMap[full]; ok && time.Now().Before(cache.expire) {
		c.xrxdMu.Unlock()
		return cache.events
	}
	c.xrxdMu.Unlock()

	var xs []*protocol.XRXD
	_ = c.pool.do(func(conn *protocol.Conn) error {
		req, e := protocol.ReqGbbq(full)
		if e != nil {
			return e
		}
		bs, e := conn.Do(req.Type, req.Data)
		if e != nil {
			return e
		}
		xs, e = protocol.DecodeXRXDs(bs)
		return e
	})

	evs := make([]xrxdEvent, 0, len(xs))
	for _, x := range xs {
		m := (10 + x.Songzhuangu + x.Peigu) / 10
		if m == 0 {
			m = 1
		}
		evs = append(evs, xrxdEvent{
			t: x.Time,
			m: m,
			c: (x.Fenhong - x.Peigu*x.Peigujia) / 10,
		})
	}
	sort.Slice(evs, func(i, j int) bool { return evs[i].t.Before(evs[j].t) })

	now := time.Now()
	expire := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(24 * time.Hour)
	c.xrxdMu.Lock()
	c.xrxdMap[full] = xrxdCache{events: evs, expire: expire}
	c.xrxdMu.Unlock()
	return evs
}

// ---- Minute ----

func (c *Client) Minute(code, date string) ([]MinutePoint, error) {
	full, err := protocol.FullCode(code)
	if err != nil {
		return nil, err
	}
	if date == "" {
		date = time.Now().Format("20060102")
	}
	var ms []protocol.MinutePoint
	err = c.pool.do(func(conn *protocol.Conn) error {
		req, e := protocol.ReqHistoryMinute(date, full)
		if e != nil {
			return e
		}
		bs, e := conn.Do(req.Type, req.Data)
		if e != nil {
			return e
		}
		ms, e = protocol.DecodeHistoryMinute(bs)
		return e
	})
	if err != nil {
		return nil, err
	}
	out := make([]MinutePoint, len(ms))
	for i, m := range ms {
		out[i] = MinutePoint{Time: m.Time, Price: m.Price.Yuan(), Volume: m.Volume}
	}
	return out, nil
}

// ---- Trades ----

func (c *Client) Trades(code, date string) ([]TradeTick, error) {
	full, err := protocol.FullCode(code)
	if err != nil {
		return nil, err
	}
	isETF := protocol.Kind(full) == "etf"

	if date == "" {
		return c.todayTrades(full, isETF)
	}
	return c.historyTrades(full, date, isETF)
}

func (c *Client) todayTrades(full string, isETF bool) ([]TradeTick, error) {
	today := time.Now().Format("20060102")
	var all []*protocol.TradeTick
	const pageSize = uint16(1800)
	for start := uint16(0); ; start += pageSize {
		var chunk []*protocol.TradeTick
		err := c.pool.do(func(conn *protocol.Conn) error {
			req, e := protocol.ReqTrade(full, start, pageSize)
			if e != nil {
				return e
			}
			bs, e := conn.Do(req.Type, req.Data)
			if e != nil {
				return e
			}
			chunk, e = protocol.DecodeTrades(bs, today, isETF)
			return e
		})
		if err != nil {
			return nil, err
		}
		all = append(chunk, all...)
		if len(chunk) < int(pageSize) {
			break
		}
	}
	return toTradeTicks(all), nil
}

func (c *Client) historyTrades(full, date string, isETF bool) ([]TradeTick, error) {
	var all []*protocol.TradeTick
	const pageSize = uint16(2000)
	for start := uint16(0); ; start += pageSize {
		var chunk []*protocol.TradeTick
		err := c.pool.do(func(conn *protocol.Conn) error {
			req, e := protocol.ReqHistoryTrade(date, full, start, pageSize)
			if e != nil {
				return e
			}
			bs, e := conn.Do(req.Type, req.Data)
			if e != nil {
				return e
			}
			chunk, e = protocol.DecodeHistoryTrades(bs, date, isETF)
			return e
		})
		if err != nil {
			return nil, err
		}
		all = append(chunk, all...)
		if len(chunk) < int(pageSize) {
			break
		}
	}
	return toTradeTicks(all), nil
}

func toTradeTicks(ts []*protocol.TradeTick) []TradeTick {
	out := make([]TradeTick, len(ts))
	for i, v := range ts {
		out[i] = TradeTick{
			Time:   v.Time.Format("2006-01-02 15:04:05"),
			Price:  v.Price.Yuan(),
			Volume: v.Volume,
			Side:   v.Side,
		}
	}
	return out
}

// ---- Codes ----

func (c *Client) Codes(kind string) ([]CodeInfo, error) {
	c.codesMu.RLock()
	defer c.codesMu.RUnlock()
	out := []CodeInfo{}
	for _, e := range c.codes {
		if e.Type == kind {
			out = append(out, CodeInfo{Code: e.Code, Name: e.Name, Market: e.Market, Type: e.Type})
		}
	}
	return out, nil
}

// Factors 返回某证券各除权事件的仿射复权系数，按日期升序。
func (c *Client) Factors(code string) ([]FactorRow, error) {
	full, err := protocol.FullCode(code)
	if err != nil {
		return nil, err
	}
	var xs []*protocol.XRXD
	err = c.pool.do(func(conn *protocol.Conn) error {
		req, e := protocol.ReqGbbq(full)
		if e != nil {
			return e
		}
		bs, e := conn.Do(req.Type, req.Data)
		if e != nil {
			return e
		}
		xs, e = protocol.DecodeXRXDs(bs)
		return e
	})
	if err != nil {
		return nil, err
	}

	// 收集事件（已按时间升序）
	type ev struct {
		x    *protocol.XRXD
		m, c float64
	}
	evs := make([]ev, 0, len(xs))
	for _, x := range xs {
		m := (10 + x.Songzhuangu + x.Peigu) / 10
		if m == 0 {
			m = 1
		}
		c := (x.Fenhong - x.Peigu*x.Peigujia) / 10
		evs = append(evs, ev{x: x, m: m, c: c})
	}

	// 即使没有 K 线数据，也能算出每条除权事件对应的累积前复权系数。
	// 将事件从最新向最早回溯（与复权展开方向一致），最新事件前的累积系数为 (1, 0)。
	n := len(evs)
	qfqMul := make([]float64, n)
	qfqAdd := make([]float64, n)
	a, b := 1.0, 0.0
	for i := n - 1; i >= 0; i-- {
		a = a / evs[i].m
		b = b - a*evs[i].c
		qfqMul[i] = a
		qfqAdd[i] = b
	}

	out := make([]FactorRow, n)
	for i, e := range evs {
		x := e.x
		out[i] = FactorRow{
			Date:       x.Time.Format("2006-01-02"),
			Cash:       round(x.Fenhong/10, 4),
			Bonus:      round(x.Songzhuangu/10, 4),
			Allot:      round(x.Peigu/10, 4),
			AllotPrice: round(x.Peigujia, 4),
			M:          round(e.m, 6),
			C:          round(e.c, 6),
			QFQMul:     round(qfqMul[i], 6),
			QFQAdd:     round(qfqAdd[i], 6),
		}
	}
	return out, nil
}

// ---- XRXD ----

func (c *Client) XRXD(code string) ([]XRXDRow, error) {
	full, err := protocol.FullCode(code)
	if err != nil {
		return nil, err
	}
	var xs []*protocol.XRXD
	err = c.pool.do(func(conn *protocol.Conn) error {
		req, e := protocol.ReqGbbq(full)
		if e != nil {
			return e
		}
		bs, e := conn.Do(req.Type, req.Data)
		if e != nil {
			return e
		}
		xs, e = protocol.DecodeXRXDs(bs)
		return e
	})
	if err != nil {
		return nil, err
	}
	out := make([]XRXDRow, len(xs))
	for i, x := range xs {
		out[i] = XRXDRow{
			Date:  x.Time.Format("2006-01-02"),
			Cash:  round(x.Fenhong/10, 4),
			Allot: round(x.Peigu/10, 4),
			Price: x.Peigujia,
			Bonus: round(x.Songzhuangu/10, 4),
		}
	}
	return out, nil
}

// ---- 周/月K合成 ----

func aggregate(day []KlineBar, period Period) []KlineBar {
	type group struct {
		key   string
		open  float64
		high  float64
		low   float64
		close float64
		last  float64
		vol   int64
		amt   float64
	}
	var cur *group
	var out []KlineBar
	flush := func() {
		if cur == nil {
			return
		}
		out = append(out, KlineBar{
			Time: cur.key, Open: cur.open, High: cur.high,
			Low: cur.low, Close: cur.close, Last: cur.last,
			Vol: cur.vol, Amt: cur.amt,
		})
		cur = nil
	}
	for _, k := range day {
		var key string
		if period == PeriodWeek {
			key = weekKey(k.Time)
		} else {
			key = k.Time[:7]
		}
		if cur == nil || cur.key != key {
			flush()
			cur = &group{key: key, open: k.Open, high: k.High, low: k.Low, last: k.Last}
		}
		if k.High > cur.high {
			cur.high = k.High
		}
		if k.Low < cur.low {
			cur.low = k.Low
		}
		cur.close = k.Close
		cur.vol += k.Vol
		cur.amt += k.Amt
	}
	flush()
	return out
}

// tail 返回最近 count 条；count<=0 表示全部。
func tail(bars []KlineBar, count int) []KlineBar {
	if count > 0 && len(bars) > count {
		return bars[len(bars)-count:]
	}
	return bars
}

func weekKey(day string) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	offset := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -offset).Format("2006-01-02")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
