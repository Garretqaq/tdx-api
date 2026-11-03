package protocol

import (
	"io"
	"math"
	"strconv"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func float32frombits(b uint32) float32 { return math.Float32frombits(b) }

// gbkDecode 解码一个 GBK 双字节字符。
func gbkDecode(lead, trail byte) (rune, bool) {
	out, err := simplifiedchinese.GBK.NewDecoder().Bytes([]byte{lead, trail})
	if err != nil || len(out) == 0 {
		return 0, false
	}
	rs := []rune(string(out))
	if len(rs) == 0 {
		return 0, false
	}
	return rs[0], true
}

// QuoteLevel 盘口一档。
type QuoteLevel struct {
	Price  Price
	Volume int
}

// Quote 五档行情快照。
type Quote struct {
	Exchange   uint8
	Code       string
	ServerTime string
	Last       Price
	Open       Price
	High       Price
	Low        Price
	Close      Price
	Volume     int64
	Amount     Price
	Intuition  int64
	InsideDish int64
	OuterDisc  int64
	Rate       float64
	Bids       [5]QuoteLevel
	Asks       [5]QuoteLevel
}

// DecodeQuotes 解析行情响应。
func DecodeQuotes(bs []byte) ([]*Quote, error) {
	if len(bs) < 4 {
		return nil, io.ErrUnexpectedEOF
	}
	bs = bs[2:] // 前 2 字节未知
	n := int(Uint16(bs))
	bs = bs[2:]

	out := make([]*Quote, 0, n)
	for i := 0; i < n; i++ {
		if len(bs) < 9 {
			return nil, io.ErrUnexpectedEOF
		}
		q := &Quote{
			Exchange: bs[0],
			Code:     string(bs[1:7]),
		}
		bs = bs[9:] // 跳过 Exchange, Code(6), 活跃度(2)

		// 价格字段为增量编码，均以当日收盘价为基准
		var closeP, last, open, high, low Price
		bs, closeP = GetPrice(bs)
		bs, last = GetPrice(bs)
		bs, open = GetPrice(bs)
		bs, high = GetPrice(bs)
		bs, low = GetPrice(bs)

		q.Close = closeP * 10
		q.Last = (last + closeP) * 10
		q.Open = (open + closeP) * 10
		q.High = (high + closeP) * 10
		q.Low = (low + closeP) * 10

		var serverTime int64
		bs, serverTime = GetVarInt(bs)
		q.ServerTime = strconv.FormatInt(serverTime, 10) // 原始整数，格式待实测确认
		bs, _ = GetVarInt(bs)                            // 保留，约等于负的收盘价
		bs, q.Volume = GetVarInt(bs)
		bs, q.Intuition = GetVarInt(bs)
		if len(bs) < 4 {
			return nil, io.ErrUnexpectedEOF
		}
		q.Amount = FromYuan(float64(math.Float32frombits(Uint32(bs))))
		bs = bs[4:]
		bs, q.InsideDish = GetVarInt(bs)
		bs, q.OuterDisc = GetVarInt(bs)
		bs, _ = GetVarInt(bs)
		bs, _ = GetVarInt(bs)

		for j := 0; j < 5; j++ {
			var buy, sell Price
			var bidVol, askVol int64
			bs, buy = GetPrice(bs)
			bs, sell = GetPrice(bs)
			bs, bidVol = GetVarInt(bs)
			bs, askVol = GetVarInt(bs)
			q.Bids[j] = QuoteLevel{Price: buy*10 + q.Close, Volume: int(bidVol)}
			q.Asks[j] = QuoteLevel{Price: sell*10 + q.Close, Volume: int(askVol)}
		}

		if len(bs) < 8 {
			return nil, io.ErrUnexpectedEOF
		}
		bs = bs[2:] // 保留
		bs, _ = GetVarInt(bs)
		bs, _ = GetVarInt(bs)
		bs, _ = GetVarInt(bs)
		bs, _ = GetVarInt(bs)
		rateRaw := Uint16(bs)
		q.Rate = float64(rateRaw) / 100
		bs = bs[4:]

		out = append(out, q)
	}
	return out, nil
}

// Kline K线。
type Kline struct {
	Time   time.Time
	Last   Price
	Open   Price
	High   Price
	Low    Price
	Close  Price
	Volume int64
	Amount Price
}

// DecodeKlines 解析K线响应。
// kindIndex 为真时多解析 4 字节（上涨/下跌家数）。
// normalizeVol 为真时成交量除以 100（分钟级K线）。
func DecodeKlines(bs []byte, klineType uint8, kindIndex, normalizeVol bool) ([]*Kline, error) {
	if len(bs) < 2 {
		return nil, io.ErrUnexpectedEOF
	}
	n := int(Uint16(bs))
	bs = bs[2:]

	normalize := normalizeVol && klineType != KlineDay && klineType != KlineWeek &&
		klineType != KlineMonth && klineType != KlineDay2

	out := make([]*Kline, 0, n)
	var last Price
	for i := 0; i < n; i++ {
		if len(bs) < 4 {
			return nil, io.ErrUnexpectedEOF
		}
		k := &Kline{Time: KlineTime(bs, klineType)}
		bs = bs[4:]

		var open, closeDelta, high, low Price
		bs, open = GetPrice(bs)
		bs, closeDelta = GetPrice(bs)
		bs, high = GetPrice(bs)
		bs, low = GetPrice(bs)

		k.Last = last
		k.Open = open + last
		k.Close = last + open + closeDelta
		k.High = open + last + high
		k.Low = open + last + low
		last = k.Close

		if len(bs) < 4 {
			return nil, io.ErrUnexpectedEOF
		}
		vol := int64(math.Float32frombits(Uint32(bs)))
		bs = bs[4:]
		if normalize && vol != 0 {
			vol /= 100
		}
		k.Volume = vol
		if len(bs) < 4 {
			return nil, io.ErrUnexpectedEOF
		}
		k.Amount = FromYuan(float64(math.Float32frombits(Uint32(bs))))
		bs = bs[4:]

		if kindIndex {
			if len(bs) < 4 {
				return nil, io.ErrUnexpectedEOF
			}
			k.Volume *= 100
			bs = bs[4:] // 上涨/下跌家数
		}
		out = append(out, k)
	}
	return out, nil
}

// MinutePoint 分时数据点。
type MinutePoint struct {
	Time   string
	Price  Price
	Volume int
}

// DecodeMinute 解析当日分时响应。
func DecodeMinute(bs []byte) ([]MinutePoint, error) {
	if len(bs) < 6 {
		return nil, io.ErrUnexpectedEOF
	}
	n := int(Uint16(bs))
	bs = bs[6:]

	out := make([]MinutePoint, 0, n)
	t := time.Date(0, 1, 1, 9, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		var price Price
		bs, price = GetPrice(bs)
		bs, _ = GetVarInt(bs)
		var vol int64
		bs, vol = GetVarInt(bs)
		if i == 120 {
			t = t.Add(2 * time.Hour)
		}
		out = append(out, MinutePoint{
			Time:   t.Add(time.Duration(i) * time.Minute).Format("15:04"),
			Price:  price,
			Volume: int(vol),
		})
	}
	return out, nil
}

// DecodeHistoryMinute 解析历史分时响应（价格是逐分钟增量，需累加并乘 10）。
func DecodeHistoryMinute(bs []byte) ([]MinutePoint, error) {
	if len(bs) < 6 {
		return nil, io.ErrUnexpectedEOF
	}
	n := int(Uint16(bs))
	bs = bs[6:]

	out := make([]MinutePoint, 0, n)
	var cumulative Price
	t := time.Date(0, 1, 1, 9, 30, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		var delta Price
		bs, delta = GetPrice(bs)
		bs, _ = GetPrice(bs)
		cumulative += delta
		var vol int64
		bs, vol = GetVarInt(bs)
		if i == 120 {
			t = t.Add(90 * time.Minute)
		}
		out = append(out, MinutePoint{
			Time:   t.Add(time.Duration(i+1) * time.Minute).Format("15:04"),
			Price:  cumulative * 10,
			Volume: int(vol),
		})
	}
	return out, nil
}

// TradeTick 分笔成交。
type TradeTick struct {
	Time   time.Time
	Price  Price
	Volume int
	Side   int
}

// DecodeTrades 解析当日分笔成交响应。date 为 "20060102"，etf 为真时价格需除以 10。
func DecodeTrades(bs []byte, date string, isETF bool) ([]*TradeTick, error) {
	if len(bs) < 2 {
		return nil, io.ErrUnexpectedEOF
	}
	n := int(Uint16(bs))
	bs = bs[2:]

	div := Price(1)
	if isETF {
		div = 10
	}

	out := make([]*TradeTick, 0, n)
	var last Price
	for i := 0; i < n; i++ {
		if len(bs) < 2 {
			return nil, io.ErrUnexpectedEOF
		}
		t, err := time.ParseInLocation("2006010215:04", date+HourMinute(bs), time.Local)
		if err != nil {
			return nil, err
		}
		bs = bs[2:]
		var delta Price
		bs, delta = GetPrice(bs)
		last += delta * 10
		var vol, status int64
		bs, vol = GetVarInt(bs)
		bs, _ = GetVarInt(bs) // 单数，不使用
		bs, status = GetVarInt(bs)
		bs, _ = GetVarInt(bs)
		out = append(out, &TradeTick{
			Time:   t,
			Price:  last / div,
			Volume: int(vol),
			Side:   int(status),
		})
	}
	return out, nil
}

// DecodeHistoryTrades 解析历史分笔成交响应（比当日少一个字段）。
func DecodeHistoryTrades(bs []byte, date string, isETF bool) ([]*TradeTick, error) {
	if len(bs) < 6 {
		return nil, io.ErrUnexpectedEOF
	}
	n := int(Uint16(bs))
	bs = bs[6:]

	div := Price(1)
	if isETF {
		div = 10
	}

	out := make([]*TradeTick, 0, n)
	var last Price
	for i := 0; i < n; i++ {
		if len(bs) < 2 {
			return nil, io.ErrUnexpectedEOF
		}
		t, err := time.ParseInLocation("2006010215:04", date+HourMinute(bs), time.Local)
		if err != nil {
			return nil, err
		}
		bs = bs[2:]
		var delta Price
		bs, delta = GetPrice(bs)
		last += delta * 10
		var vol, status int64
		bs, vol = GetVarInt(bs)
		bs, status = GetVarInt(bs)
		bs, _ = GetVarInt(bs)
		out = append(out, &TradeTick{
			Time:   t,
			Price:  last / div,
			Volume: int(vol),
			Side:   int(status),
		})
	}
	return out, nil
}

// Code 证券代码条目。
type Code struct {
	Code     string
	Name     string
	Multiple uint16
	Decimal  int8
}

// DecodeCodes 解析证券代码列表响应。
func DecodeCodes(bs []byte) ([]*Code, error) {
	if len(bs) < 2 {
		return nil, io.ErrUnexpectedEOF
	}
	n := int(Uint16(bs))
	bs = bs[2:]

	out := make([]*Code, 0, n)
	for i := 0; i < n; i++ {
		if len(bs) < 29 {
			return nil, io.ErrUnexpectedEOF
		}
		out = append(out, &Code{
			Code:     string(bs[:6]),
			Multiple: Uint16(bs[6:8]),
			Name:     GBK(bs[8:16]),
			Decimal:  int8(bs[20]),
		})
		bs = bs[29:]
	}
	return out, nil
}

// XRXD 一条除权除息记录。
type XRXD struct {
	Code        string
	Time        time.Time
	Fenhong     float64 // 每 10 股派现（元）
	Peigujia    float64 // 配股价（元）
	Songzhuangu float64 // 每 10 股送转股数
	Peigu       float64 // 每 10 股配股数
}

// DecodeXRXDs 解析除权除息响应，只保留 Category=1（除权除息）的记录，按时间升序。
func DecodeXRXDs(bs []byte) ([]*XRXD, error) {
	if len(bs) < 11 {
		return nil, io.ErrUnexpectedEOF
	}
	n := int(Uint16(bs[9:11]))
	bs = bs[11:]

	out := make([]*XRXD, 0, n)
	for i := 0; i < n; i++ {
		if len(bs) < 29 {
			return nil, io.ErrUnexpectedEOF
		}
		code := ExchangeName(bs[0]) + string(bs[1:7])
		t := klineDate(bs[8:12])
		category := bs[12]
		bs = bs[13:]

		var c1, c2, c3, c4 float64
		switch category {
		case 1:
			c1 = float64(math.Float32frombits(Uint32(bs[0:4])))
			c2 = float64(math.Float32frombits(Uint32(bs[4:8])))
			c3 = float64(math.Float32frombits(Uint32(bs[8:12])))
			c4 = float64(math.Float32frombits(Uint32(bs[12:16])))
		case 11, 12:
			c3 = float64(math.Float32frombits(Uint32(bs[8:12])))
		case 13, 14:
			c1 = float64(math.Float32frombits(Uint32(bs[0:4])))
			c3 = float64(math.Float32frombits(Uint32(bs[8:12])))
		default:
			c1 = float64(math.Float32frombits(Uint32(bs[0:4])))
			c2 = float64(math.Float32frombits(Uint32(bs[4:8])))
			c3 = float64(math.Float32frombits(Uint32(bs[8:12])))
			c4 = float64(math.Float32frombits(Uint32(bs[12:16])))
		}
		bs = bs[16:]

		if category != 1 {
			continue
		}
		// float32 转 float64 带来的噪声，按 2 位小数取整（与通达信一致）
		r2 := func(v float64) float64 { return math.Round(v*100) / 100 }
		c1, c2, c3, c4 = r2(c1), r2(c2), r2(c3), r2(c4)
		out = append(out, &XRXD{
			Code:        code,
			Time:        t,
			Fenhong:     c1,
			Peigujia:    c2,
			Songzhuangu: c3,
			Peigu:       c4,
		})
	}

	// 按时间升序
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Time.Before(out[j-1].Time); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}

// klineDate 解析 4 字节日期（YYYYMMDD）。
func klineDate(bs []byte) time.Time {
	n := Uint32(bs)
	return time.Date(int(n/10000), time.Month(int(n%10000)/100), int(n%100), 15, 0, 0, 0, time.Local)
}

// Count 市场证券数量。
type Count struct{ Count uint16 }

// DecodeCount 解析证券数量响应。
func DecodeCount(bs []byte) (*Count, error) {
	if len(bs) < 2 {
		return nil, io.ErrUnexpectedEOF
	}
	return &Count{Count: Uint16(bs)}, nil
}
