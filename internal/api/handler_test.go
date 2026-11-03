package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Garretqaq/tdx-api/internal/tdx"
)

// fakeProvider 用假数据实现 tdx.Provider，供 HTTP 层单测使用。
type fakeProvider struct {
	quotesErr error
	quotes    []tdx.Quote
	klines    []tdx.KlineBar
	xrxd      []tdx.XRXDRow
	codes     []tdx.CodeInfo

	lastPeriod tdx.Period
	lastAdjust tdx.Adjust
	lastCount  int
	lastDate   string
}

func (f *fakeProvider) Quotes(codes []string) ([]tdx.Quote, error) {
	if f.quotesErr != nil {
		return nil, f.quotesErr
	}
	if f.quotes != nil {
		return f.quotes, nil
	}
	out := make([]tdx.Quote, len(codes))
	for i, c := range codes {
		out[i] = tdx.Quote{Code: c, Name: "测试", Close: 10.23, Last: 10.0}
	}
	return out, nil
}

func (f *fakeProvider) Klines(code string, period tdx.Period, adjust tdx.Adjust, count int) ([]tdx.KlineBar, error) {
	f.lastPeriod, f.lastAdjust, f.lastCount = period, adjust, count
	return f.klines, nil
}

func (f *fakeProvider) Minute(code, date string) ([]tdx.MinutePoint, error) {
	f.lastDate = date
	return []tdx.MinutePoint{{Time: "09:31", Price: 10.1, Volume: 100}}, nil
}

func (f *fakeProvider) Trades(code, date string) ([]tdx.TradeTick, error) {
	f.lastDate = date
	return []tdx.TradeTick{{Time: "2026-11-03 09:31:05", Price: 10.1, Volume: 5, Side: 0}}, nil
}

func (f *fakeProvider) Codes(kind string) ([]tdx.CodeInfo, error) { return f.codes, nil }

func (f *fakeProvider) XRXD(code string) ([]tdx.XRXDRow, error) { return f.xrxd, nil }

func (f *fakeProvider) Factors(code string) ([]tdx.FactorRow, error) {
	return []tdx.FactorRow{{Code: "sh600000", Date: "2026-07-16", Cash: 0.42, M: 1, C: 0.042, QFQMul: 0.95, QFQAdd: -0.42}}, nil
}

func do(h http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) Resp {
	t.Helper()
	var r Resp
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("响应不是合法 JSON: %v，原文 %s", err, w.Body.String())
	}
	return r
}

func TestHealthzNoAuth(t *testing.T) {
	h := New(&fakeProvider{}, []string{"secret"})
	w := do(h, "/healthz", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz 应免鉴权，得到 %d", w.Code)
	}
}

func TestAuthMissingKey(t *testing.T) {
	h := New(&fakeProvider{}, []string{"secret"})
	w := do(h, "/api/v1/quote?codes=600000", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("缺少 key 应返回 401，得到 %d", w.Code)
	}
}

func TestAuthWrongKey(t *testing.T) {
	h := New(&fakeProvider{}, []string{"secret"})
	w := do(h, "/api/v1/quote?codes=600000", map[string]string{"X-API-Key": "wrong"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("错误 key 应返回 401，得到 %d", w.Code)
	}
}

func TestAuthValidKey(t *testing.T) {
	h := New(&fakeProvider{}, []string{"secret"})
	w := do(h, "/api/v1/quote?codes=600000", map[string]string{"X-API-Key": "secret"})
	if w.Code != http.StatusOK {
		t.Fatalf("正确 key 应返回 200，得到 %d", w.Code)
	}
	r := decode(t, w)
	if r.Code != 0 {
		t.Fatalf("业务码应为 0，得到 %d", r.Code)
	}
}

func TestAuthQueryKey(t *testing.T) {
	h := New(&fakeProvider{}, []string{"secret"})
	w := do(h, "/api/v1/quote?codes=600000&api_key=secret", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("query 传 key 应放行，得到 %d", w.Code)
	}
}

func TestNoKeysConfiguredAllowsAll(t *testing.T) {
	h := New(&fakeProvider{}, nil)
	w := do(h, "/api/v1/quote?codes=600000", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("未配置 key 时应放行，得到 %d", w.Code)
	}
}

func TestQuoteMissingCodes(t *testing.T) {
	h := New(&fakeProvider{}, nil)
	w := do(h, "/api/v1/quote", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺少 codes 应返回 400，得到 %d", w.Code)
	}
}

func TestQuoteTooManyCodes(t *testing.T) {
	h := New(&fakeProvider{}, nil)
	path := "/api/v1/quote?codes="
	for i := 0; i < 51; i++ {
		if i > 0 {
			path += ","
		}
		path += "600000"
	}
	w := do(h, path, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("超过 50 只应返回 400，得到 %d", w.Code)
	}
}

func TestQuoteUpstreamError(t *testing.T) {
	h := New(&fakeProvider{quotesErr: errors.New("上游超时")}, nil)
	w := do(h, "/api/v1/quote?codes=600000", nil)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("上游错误应返回 502，得到 %d", w.Code)
	}
}

func TestKlineDefaults(t *testing.T) {
	f := &fakeProvider{klines: []tdx.KlineBar{{Time: "2026-11-03", Close: 10.09}}}
	h := New(f, nil)
	w := do(h, "/api/v1/kline?code=600000", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("应返回 200，得到 %d", w.Code)
	}
	if f.lastPeriod != tdx.PeriodDay {
		t.Errorf("period 默认应为 day，得到 %s", f.lastPeriod)
	}
	if f.lastAdjust != tdx.AdjustNone {
		t.Errorf("adjust 默认应为 none，得到 %s", f.lastAdjust)
	}
	if f.lastCount != 100 {
		t.Errorf("count 默认应为 100，得到 %d", f.lastCount)
	}
}

// assertRowCode 校验响应 data 首行带有 code 字段，下游面板靠它映射 symbol。
func assertRowCode(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var r struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if len(r.Data) == 0 {
		t.Fatal("data 为空")
	}
	if got := r.Data[0]["code"]; got != want {
		t.Fatalf("首行 code = %v，期望 %s", got, want)
	}
}

func TestKlineResponseHasCode(t *testing.T) {
	f := &fakeProvider{klines: []tdx.KlineBar{{Code: "sh600000", Time: "2026-11-03", Close: 10.09}}}
	w := do(New(f, nil), "/api/v1/kline?code=600000", nil)
	assertRowCode(t, w, "sh600000")
}

func TestFactorResponseHasCode(t *testing.T) {
	w := do(New(&fakeProvider{}, nil), "/api/v1/factor?code=600000", nil)
	assertRowCode(t, w, "sh600000")
}

func levels(pv ...float64) []tdx.Level {
	out := make([]tdx.Level, 0, len(pv)/2)
	for i := 0; i+1 < len(pv); i += 2 {
		out = append(out, tdx.Level{Price: pv[i], Volume: int(pv[i+1])})
	}
	return out
}

func TestDepth5Nested(t *testing.T) {
	f := &fakeProvider{quotes: []tdx.Quote{
		{
			Code: "sh600000",
			Bids: levels(10.09, 778, 10.08, 468, 10.07, 279, 10.06, 699, 10.05, 238),
			Asks: levels(10.1, 179, 10.11, 1524, 10.12, 91, 10.13, 2695, 10.14, 220),
		},
		{Code: "sh000001", Bids: []tdx.Level{}, Asks: []tdx.Level{}},
	}}
	w := do(New(f, nil), "/api/v1/depth5?codes=600000,sh000001&format=dot", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("应返回 200，得到 %d", w.Code)
	}
	var r struct {
		Data []struct {
			Code string       `json:"code"`
			Bids [][2]float64 `json:"bids"`
			Asks [][2]float64 `json:"asks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if len(r.Data) != 2 {
		t.Fatalf("应返回 2 行，得到 %d", len(r.Data))
	}
	stock, index := r.Data[0], r.Data[1]
	if stock.Code != "600000.SH" {
		t.Errorf("code = %q，期望 600000.SH", stock.Code)
	}
	if len(stock.Bids) != 5 || len(stock.Asks) != 5 {
		t.Fatalf("五档不完整: bids=%d asks=%d", len(stock.Bids), len(stock.Asks))
	}
	if stock.Bids[0] != [2]float64{10.09, 778} {
		t.Errorf("买一 = %v，期望 [10.09 778]", stock.Bids[0])
	}
	if stock.Asks[4] != [2]float64{10.14, 220} {
		t.Errorf("卖五 = %v，期望 [10.14 220]", stock.Asks[4])
	}
	// 指数无盘口：必须是空数组而不是 null，否则面板会把 null 当缺失列
	if index.Code != "000001.SH" {
		t.Errorf("指数 code = %q，期望 000001.SH", index.Code)
	}
	if len(index.Bids) != 0 || len(index.Asks) != 0 {
		t.Errorf("指数盘口应为空数组，得到 bids=%v asks=%v", index.Bids, index.Asks)
	}
	var raw struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"bids", "asks"} {
		if got := string(raw.Data[1][k]); got != "[]" {
			t.Errorf("指数 %s 序列化为 %s，期望 []", k, got)
		}
	}
}

func TestDepth5MissingCodes(t *testing.T) {
	w := do(New(&fakeProvider{}, nil), "/api/v1/depth5", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺少 codes 应返回 400，得到 %d", w.Code)
	}
}

func TestKlineParams(t *testing.T) {
	f := &fakeProvider{}
	h := New(f, nil)
	w := do(h, "/api/v1/kline?code=sh600000&period=week&adjust=qfq&count=0", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("应返回 200，得到 %d", w.Code)
	}
	if f.lastPeriod != tdx.PeriodWeek || f.lastAdjust != tdx.AdjustQFQ || f.lastCount != 0 {
		t.Fatalf("参数未正确传递: period=%s adjust=%s count=%d", f.lastPeriod, f.lastAdjust, f.lastCount)
	}
}

func TestKlineBadPeriod(t *testing.T) {
	h := New(&fakeProvider{}, nil)
	w := do(h, "/api/v1/kline?code=600000&period=2h", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 period 应返回 400，得到 %d", w.Code)
	}
}

func TestKlineBadAdjust(t *testing.T) {
	h := New(&fakeProvider{}, nil)
	w := do(h, "/api/v1/kline?code=600000&adjust=abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 adjust 应返回 400，得到 %d", w.Code)
	}
}

func TestKlineNegativeCount(t *testing.T) {
	h := New(&fakeProvider{}, nil)
	w := do(h, "/api/v1/kline?code=600000&count=-1", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("负数 count 应返回 400，得到 %d", w.Code)
	}
}

func TestKlineDotCode(t *testing.T) {
	f := &fakeProvider{klines: []tdx.KlineBar{{Code: "sh600000", Time: "2026-11-03", Close: 10.09}}}
	w := do(New(f, nil), "/api/v1/kline?code=sh600000&format=dot", nil)
	assertRowCode(t, w, "600000.SH")
}

func TestKlineAcceptsDotCode(t *testing.T) {
	f := &fakeProvider{klines: []tdx.KlineBar{{Code: "sh600000", Time: "2026-11-03", Close: 10.09}}}
	w := do(New(f, nil), "/api/v1/kline?code=sh600000", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("dot 写法应被接受，得到 %d", w.Code)
	}
}

func TestCodesDefaultType(t *testing.T) {
	f := &fakeProvider{codes: []tdx.CodeInfo{{Code: "sh600000", Name: "浦发银行", Type: "stock"}}}
	h := New(f, nil)
	w := do(h, "/api/v1/codes", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("应返回 200，得到 %d", w.Code)
	}
	r := decode(t, w)
	if r.Data == nil {
		t.Fatal("data 不应为空")
	}
}

func TestKlineMinutePeriodRejectsAdjust(t *testing.T) {
	h := New(&fakeProvider{}, nil)
	w := do(h, "/api/v1/kline?code=600000&period=1m&adjust=qfq", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("分钟K线要复权应返回 400，得到 %d", w.Code)
	}
}

func TestKlineMinutePeriodNoAdjustOK(t *testing.T) {
	h := New(&fakeProvider{}, nil)
	w := do(h, "/api/v1/kline?code=600000&period=1m", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("分钟K线不复权应返回 200，得到 %d", w.Code)
	}
}

func TestCodesBadType(t *testing.T) {
	h := New(&fakeProvider{}, nil)
	w := do(h, "/api/v1/codes?type=bond", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 type 应返回 400，得到 %d", w.Code)
	}
}

func TestMinuteDatePassThrough(t *testing.T) {
	f := &fakeProvider{}
	h := New(f, nil)
	do(h, "/api/v1/minute?code=600000&date=20261102", nil)
	if f.lastDate != "20261102" {
		t.Fatalf("date 应透传，得到 %q", f.lastDate)
	}
}

func TestXRXDPath(t *testing.T) {
	f := &fakeProvider{xrxd: []tdx.XRXDRow{{Date: "2026-07-16", Cash: 0.42}}}
	h := New(f, nil)
	w := do(h, "/api/v1/xrxd?code=600000", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("应返回 200，得到 %d", w.Code)
	}
}
