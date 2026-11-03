package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Garretqaq/tdx-api/internal/tdx"
)

// New 构造路由。/healthz 不鉴权，其余走 apiKeyAuth。
func New(p tdx.Provider, keys []string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"status": "ok"})
	})

	api := http.NewServeMux()
	api.HandleFunc("GET /api/v1/quote", handleQuote(p))
	api.HandleFunc("GET /api/v1/orderbook", handleOrderbook(p))
	api.HandleFunc("GET /api/v1/kline", handleKline(p))
	api.HandleFunc("GET /api/v1/minute", handleMinute(p))
	api.HandleFunc("GET /api/v1/trade", handleTrade(p))
	api.HandleFunc("GET /api/v1/codes", handleCodes(p))
	api.HandleFunc("GET /api/v1/xrxd", handleXRXD(p))
	api.HandleFunc("GET /api/v1/factor", handleFactor(p))

	mux.Handle("/api/", apiKeyAuth(keys, api))
	return mux
}

func handleQuote(p tdx.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		qs, done := fetchQuotes(w, r, p)
		if done {
			return
		}
		ok(w, qs)
	}
}

func handleOrderbook(p tdx.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		qs, done := fetchQuotes(w, r, p)
		if done {
			return
		}
		out := make([]tdx.Orderbook, len(qs))
		for i, q := range qs {
			out[i] = tdx.ToOrderbook(q)
		}
		ok(w, out)
	}
}

// fetchQuotes 解析 codes 参数并查询行情快照；出错时已写响应，返回 done=true。
func fetchQuotes(w http.ResponseWriter, r *http.Request, p tdx.Provider) ([]tdx.Quote, bool) {
	codes := splitCodes(strings.TrimSpace(r.URL.Query().Get("codes")))
	if len(codes) == 0 {
		fail(w, http.StatusBadRequest, "参数 codes 不能为空")
		return nil, true
	}
	if len(codes) > 50 {
		fail(w, http.StatusBadRequest, "单次最多查询 50 只")
		return nil, true
	}
	qs, err := p.Quotes(codes)
	if err != nil {
		fail(w, http.StatusBadGateway, err.Error())
		return nil, true
	}
	return qs, false
}

func handleKline(p tdx.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			fail(w, http.StatusBadRequest, "参数 code 不能为空")
			return
		}
		period := tdx.Period(strings.ToLower(strings.TrimSpace(r.URL.Query().Get("period"))))
		if period == "" {
			period = tdx.PeriodDay
		}
		if !validPeriod(period) {
			fail(w, http.StatusBadRequest, "period 取值: 1m/5m/15m/30m/60m/day/week/month")
			return
		}
		adjust := tdx.AdjustNone
		if v := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("adjust"))); v != "" {
			adjust = tdx.Adjust(v)
			if !validAdjust(adjust) {
				fail(w, http.StatusBadRequest, "adjust 取值: none/qfq/hfq")
				return
			}
			if adjust != tdx.AdjustNone && isMinutePeriod(period) {
				fail(w, http.StatusBadRequest, "分钟K线不支持复权，请将 adjust 设为 none")
				return
			}
		}
		count := 100
		if v := strings.TrimSpace(r.URL.Query().Get("count")); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				fail(w, http.StatusBadRequest, "count 须为非负整数")
				return
			}
			count = n
		}
		bars, err := p.Klines(code, period, adjust, count)
		if err != nil {
			fail(w, http.StatusBadGateway, err.Error())
			return
		}
		ok(w, bars)
	}
}

func handleMinute(p tdx.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			fail(w, http.StatusBadRequest, "参数 code 不能为空")
			return
		}
		date := strings.TrimSpace(r.URL.Query().Get("date"))
		ms, err := p.Minute(code, date)
		if err != nil {
			fail(w, http.StatusBadGateway, err.Error())
			return
		}
		ok(w, ms)
	}
}

func handleTrade(p tdx.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			fail(w, http.StatusBadRequest, "参数 code 不能为空")
			return
		}
		date := strings.TrimSpace(r.URL.Query().Get("date"))
		ts, err := p.Trades(code, date)
		if err != nil {
			fail(w, http.StatusBadGateway, err.Error())
			return
		}
		ok(w, ts)
	}
}

func handleCodes(p tdx.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
		switch kind {
		case "stock", "etf", "index":
		case "":
			kind = "stock"
		default:
			fail(w, http.StatusBadRequest, "type 取值: stock/etf/index")
			return
		}
		cs, err := p.Codes(kind)
		if err != nil {
			fail(w, http.StatusBadGateway, err.Error())
			return
		}
		ok(w, cs)
	}
}

func handleXRXD(p tdx.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			fail(w, http.StatusBadRequest, "参数 code 不能为空")
			return
		}
		xs, err := p.XRXD(code)
		if err != nil {
			fail(w, http.StatusBadGateway, err.Error())
			return
		}
		ok(w, xs)
	}
}

func handleFactor(p tdx.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			fail(w, http.StatusBadRequest, "参数 code 不能为空")
			return
		}
		fs, err := p.Factors(code)
		if err != nil {
			fail(w, http.StatusBadGateway, err.Error())
			return
		}
		ok(w, fs)
	}
}

func splitCodes(s string) []string {
	out := []string{}
	for _, c := range strings.Split(s, ",") {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	return out
}

func validPeriod(p tdx.Period) bool {
	switch p {
	case tdx.Period1Min, tdx.Period5Min, tdx.Period15Min,
		tdx.Period30Min, tdx.Period60Min, tdx.PeriodDay,
		tdx.PeriodWeek, tdx.PeriodMonth:
		return true
	}
	return false
}

func validAdjust(a tdx.Adjust) bool {
	switch a {
	case tdx.AdjustNone, tdx.AdjustQFQ, tdx.AdjustHFQ:
		return true
	}
	return false
}

func isMinutePeriod(p tdx.Period) bool {
	switch p {
	case tdx.Period1Min, tdx.Period5Min, tdx.Period15Min, tdx.Period30Min, tdx.Period60Min:
		return true
	}
	return false
}
