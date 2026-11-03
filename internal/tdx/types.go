// Package tdx 封装对通达信行情服务器的访问，定义本服务自己的数据类型。
package tdx

import "strings"

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

// Depth5Row 五档盘口，格式对齐行情面板的 depth5 数据集契约：
// bids/asks 为 [[价, 量], ...]，按一档到五档排列，价格单位元、量单位手。
// 指数没有真实盘口，bids/asks 为空数组。
type Depth5Row struct {
	Code string       `json:"code"` // 代码，写法由请求参数 format 决定
	Bids [][2]float64 `json:"bids"`
	Asks [][2]float64 `json:"asks"`
}

// ToDepth5 把行情快照的盘口转成 depth5 行。
func ToDepth5(q Quote) Depth5Row {
	row := Depth5Row{Code: q.Code, Bids: [][2]float64{}, Asks: [][2]float64{}}
	for _, l := range q.Bids {
		row.Bids = append(row.Bids, [2]float64{l.Price, float64(l.Volume)})
	}
	for _, l := range q.Asks {
		row.Asks = append(row.Asks, [2]float64{l.Price, float64(l.Volume)})
	}
	return row
}

// DotCode 把 "sh600000" 转成 "600000.SH"；其他写法原样返回。
func DotCode(full string) string {
	if len(full) != 8 {
		return full
	}
	return full[2:] + "." + strings.ToUpper(full[:2])
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
	Code       string   `json:"code"`        // 带市场前缀，如 sh600000
	Date       string   `json:"date"`        // 除权日 "2006-01-02"
	Cash       float64  `json:"cash"`        // 每股派现（元）
	Bonus      float64  `json:"bonus"`       // 每股送转
	Allot      float64  `json:"allot"`       // 每股配股
	AllotPrice float64  `json:"allot_price"` // 配股价（元）
	M          float64  `json:"m"`           // 单次乘法因子
	C          float64  `json:"c"`           // 单次加法偏移（每股净现金流出，元）
	QFQMul     float64  `json:"qfq_mul"`     // 该事件之前历史价格的累积前复权乘数
	QFQAdd     float64  `json:"qfq_add"`     // 累积前复权加法偏移（元）
	// ExFactor 单事件比值 = 除权前收盘 / 除权参考价，与行情面板的 adj_factor
	// 契约一致（非累积）。拿不到前收盘时为 null。
	ExFactor *float64 `json:"ex_factor"`
}

// CodeInfo 证券代码信息。
type CodeInfo struct {
	Code   string `json:"code"`   // 带市场前缀，如 sh600000
	Name   string `json:"name"`   // 名称
	Market string `json:"market"` // sh / sz / bj
	Type   string `json:"type"`   // stock / etf / index
}
