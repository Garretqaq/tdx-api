package tdx

// Provider 通达信数据访问接口，HTTP 层依赖它。
// 便于单元测试使用假实现。
type Provider interface {
	// Quotes 返回多条行情快照，顺序与请求的 codes 一致。
	Quotes(codes []string) ([]Quote, error)
	// Klines 返回K线。adjust 为复权方式；count<=0 表示全量历史。
	Klines(code string, period Period, adjust Adjust, count int) ([]KlineBar, error)
	// Minute 返回分时。date 为空表示当日，格式 "20060102"。
	Minute(code, date string) ([]MinutePoint, error)
	// Trades 返回分笔成交。date 为空表示当日，格式 "20060102"。
	Trades(code, date string) ([]TradeTick, error)
	// Codes 返回某类证券的代码列表。
	Codes(kind string) ([]CodeInfo, error)
	// XRXD 返回某证券的除权除息记录，按日期升序。
	XRXD(code string) ([]XRXDRow, error)
}
