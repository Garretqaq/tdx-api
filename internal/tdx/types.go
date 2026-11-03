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

// KlineBar 一根K线。
type KlineBar struct {
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

// CodeInfo 证券代码信息。
type CodeInfo struct {
	Code   string `json:"code"`   // 带市场前缀，如 sh600000
	Name   string `json:"name"`   // 名称
	Market string `json:"market"` // sh / sz / bj
	Type   string `json:"type"`   // stock / etf / index
}
