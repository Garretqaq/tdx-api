// Package tdx 封装对通达信行情服务器的访问，定义本服务自己的数据类型。
package tdx

// Quote 行情快照。
type Quote struct {
	Code       string  `json:"code"`        // 带市场前缀，如 sh600000
	Name       string  `json:"name"`        // 证券名称，可能为空
	Time       string  `json:"time"`        // 服务器时间，北京时间 "2006-01-02 15:04:05"
	Last       float64 `json:"last"`        // 昨收价，单位元
	Open       float64 `json:"open"`        // 开盘价，单位元
	High       float64 `json:"high"`        // 最高价，单位元
	Low        float64 `json:"low"`         // 最低价，单位元
	Close      float64 `json:"close"`       // 最新价，单位元
	Volume     int64   `json:"volume"`      // 成交量，单位手
	Amount     float64 `json:"amount"`      // 成交额，单位元
	Intuition  int     `json:"intuition"`   // 现量
	InsideDish int     `json:"inside_dish"` // 内盘
	OuterDisc  int     `json:"outer_disc"`  // 外盘
	Bids       []Level `json:"bids"`        // 买盘 1-5
	Asks       []Level `json:"asks"`        // 卖盘 1-5
}

// Level 盘口一档。
type Level struct {
	Price  float64 `json:"price"` // 单位元
	Volume int     `json:"volume"`
}

// Orderbook 扁平化的五档盘口，供只支持平面字段映射的调用方使用。
// 缺失的档位（如指数）为 null。
type Orderbook struct {
	Code       string   `json:"code"` // 带市场前缀，如 sh600000
	Bid1Price  *float64 `json:"bid1_price"`
	Bid1Volume *int     `json:"bid1_volume"`
	Bid2Price  *float64 `json:"bid2_price"`
	Bid2Volume *int     `json:"bid2_volume"`
	Bid3Price  *float64 `json:"bid3_price"`
	Bid3Volume *int     `json:"bid3_volume"`
	Bid4Price  *float64 `json:"bid4_price"`
	Bid4Volume *int     `json:"bid4_volume"`
	Bid5Price  *float64 `json:"bid5_price"`
	Bid5Volume *int     `json:"bid5_volume"`
	Ask1Price  *float64 `json:"ask1_price"`
	Ask1Volume *int     `json:"ask1_volume"`
	Ask2Price  *float64 `json:"ask2_price"`
	Ask2Volume *int     `json:"ask2_volume"`
	Ask3Price  *float64 `json:"ask3_price"`
	Ask3Volume *int     `json:"ask3_volume"`
	Ask4Price  *float64 `json:"ask4_price"`
	Ask4Volume *int     `json:"ask4_volume"`
	Ask5Price  *float64 `json:"ask5_price"`
	Ask5Volume *int     `json:"ask5_volume"`
}

// ToOrderbook 把行情快照的盘口摊平。价格单位元，量单位手。
func ToOrderbook(q Quote) Orderbook {
	o := Orderbook{Code: q.Code}
	bids := [5]struct {
		p **float64
		v **int
	}{
		{&o.Bid1Price, &o.Bid1Volume}, {&o.Bid2Price, &o.Bid2Volume}, {&o.Bid3Price, &o.Bid3Volume},
		{&o.Bid4Price, &o.Bid4Volume}, {&o.Bid5Price, &o.Bid5Volume},
	}
	asks := [5]struct {
		p **float64
		v **int
	}{
		{&o.Ask1Price, &o.Ask1Volume}, {&o.Ask2Price, &o.Ask2Volume}, {&o.Ask3Price, &o.Ask3Volume},
		{&o.Ask4Price, &o.Ask4Volume}, {&o.Ask5Price, &o.Ask5Volume},
	}
	for i := 0; i < len(q.Bids) && i < 5; i++ {
		l := q.Bids[i]
		*bids[i].p, *bids[i].v = &l.Price, &l.Volume
	}
	for i := 0; i < len(q.Asks) && i < 5; i++ {
		l := q.Asks[i]
		*asks[i].p, *asks[i].v = &l.Price, &l.Volume
	}
	return o
}

// KlineBar 一根K线。
type KlineBar struct {
	Code  string  `json:"code"`  // 带市场前缀，如 sh600000
	Time  string  `json:"time"`  // 北京时间 "2006-01-02 15:04"；日/周/月为 "2006-01-02"
	Open  float64 `json:"open"`  // 单位元
	High  float64 `json:"high"`  // 单位元
	Low   float64 `json:"low"`   // 单位元
	Close float64 `json:"close"` // 单位元
	Last  float64 `json:"last"`  // 昨收价，单位元
	Vol   int64   `json:"vol"`   // 成交量，单位手
	Amt   float64 `json:"amt"`   // 成交额，单位元
}

// Period K线周期。
type Period string

const (
	Period1Min  Period = "1m"
	Period5Min  Period = "5m"
	Period15Min Period = "15m"
	Period30Min Period = "30m"
	Period60Min Period = "60m"
	PeriodDay   Period = "day"
	PeriodWeek  Period = "week"
	PeriodMonth Period = "month"
)

// Adjust 复权方式。
type Adjust string

const (
	AdjustNone Adjust = "none"
	AdjustQFQ  Adjust = "qfq"
	AdjustHFQ  Adjust = "hfq"
)

// MinutePoint 分时数据点。
type MinutePoint struct {
	Time   string  `json:"time"`   // "15:04"
	Price  float64 `json:"price"`  // 单位元
	Volume int     `json:"volume"` // 该分钟成交量，单位手
}

// TradeTick 分笔成交。
type TradeTick struct {
	Time   string  `json:"time"`   // 北京时间 "2006-01-02 15:04:05"
	Price  float64 `json:"price"`  // 单位元
	Volume int     `json:"volume"` // 单位手
	Side   int     `json:"side"`   // 0=买 1=卖 2=中性
}

// XRXDRow 一条除权除息记录。
type XRXDRow struct {
	Date  string  `json:"date"`  // "2006-01-02"
	Cash  float64 `json:"cash"`  // 每股派现，单位元
	Allot float64 `json:"allot"` // 每股配股数
	Price float64 `json:"price"` // 配股价，单位元
	Bonus float64 `json:"bonus"` // 每股送转股数
}

// FactorRow 一条除权事件的仿射复权系数。
// 前复权价格 = round2(QFQMul * raw_price + QFQAdd)
type FactorRow struct {
	Code       string  `json:"code"`        // 带市场前缀，如 sh600000
	Date       string  `json:"date"`        // 除权日 "2006-01-02"
	Cash       float64 `json:"cash"`        // 每股派现（元）
	Bonus      float64 `json:"bonus"`       // 每股送转
	Allot      float64 `json:"allot"`       // 每股配股
	AllotPrice float64 `json:"allot_price"` // 配股价（元）
	M          float64 `json:"m"`           // 单次乘法因子
	C          float64 `json:"c"`           // 单次加法偏移（每股净现金流出，元）
	QFQMul     float64 `json:"qfq_mul"`     // 该事件之前历史价格的累积前复权乘数
	QFQAdd     float64 `json:"qfq_add"`     // 累积前复权加法偏移（元）
}

// CodeInfo 证券代码信息。
type CodeInfo struct {
	Code   string `json:"code"`   // 带市场前缀，如 sh600000
	Name   string `json:"name"`   // 名称
	Market string `json:"market"` // sh / sz / bj
	Type   string `json:"type"`   // stock / etf / index
}
