# tdx-api

代理通达信行情服务器，以 HTTP API 形式提供 A 股（沪、深）行情。零第三方行情库依赖，自行实现通达信二进制协议。

## 运行

### Docker Compose（推荐）

```bash
cp .env.example .env      # 填入自己的 key
vim .env
docker compose pull       # 从 GHCR 拉取镜像
docker compose up -d
docker compose logs -f                                  # 看日志
docker compose down                                     # 停止
docker compose pull && docker compose up -d             # 更新到最新镜像
```

镜像已由 GitHub Actions 构建（amd64/arm64）并推送到 GHCR，compose 只拉不构建。
若拉取报权限错误，先把 GHCR 包设为 public，或执行 `docker login ghcr.io`。

### Docker

```bash
docker run -d -p 8080:8080 -e TDX_API_KEYS=key1,key2 ghcr.io/garretqaq/tdx-api:latest
```

### 本地直接运行

```bash
go run .          # 自动读取当前目录的 .env
```

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `TDX_API_KEYS` | 空 | 逗号分隔的 API Key；为空时不鉴权（仅建议本地调试） |
| `TDX_LISTEN_ADDR` | `:8080` | 监听地址 |
| `TDX_POOL_SIZE` | `4` | 通达信连接池大小 |
| `TDX_TIMEOUT` | `30s` | 单次请求通达信的超时 |

鉴权：请求头 `X-API-Key: <key>`，也支持 query `api_key=<key>`。`/healthz` 不鉴权。

## 接口

统一返回 `{"code":0,"message":"success","data":...}`；错误时 HTTP 状态码如实返回（400 参数错误、401 未授权、502 通达信上游出错），`code` 为 -1。价格单位均为元。

```
GET /api/v1/quote?codes=600000,510300,sh000001      # 五档行情，最多 50 只
GET /api/v1/orderbook?codes=600000,510300           # 扁平五档盘口 bid1_price…ask5_volume，最多 50 只
GET /api/v1/kline?code=600000&period=day&adjust=qfq&count=100
GET /api/v1/minute?code=600000[&date=20261102]      # 分时，不传 date 为当日
GET /api/v1/trade?code=600000[&date=20261102]       # 分笔成交，不传 date 为当日
GET /api/v1/codes?type=stock|etf|index
GET /api/v1/xrxd?code=600000                        # 除权除息
GET /healthz
```

- `period`：`1m/5m/15m/30m/60m/day/week/month`，默认 `day`
- `adjust`：`none/qfq/hfq`，默认 `none`；分钟 K 线不支持复权
- `count`：默认 100，`0` 表示全量历史
- 代码支持 `600000` 和 `sh600000` 两种写法，返回统一带前缀
- 当日分笔成交需盘后调用，盘中分页读取可能丢失间隔内的成交

## 已知限制

- 北交所不支持代码表协议，`/codes` 只含沪深；北交所个股的行情、K 线可直接按代码查询
- 行情快照的 `time` 字段为空（通达信返回的时间编码未确认）
- 指数的 `bids`/`asks` 为空数组，`/orderbook` 中对应字段为 `null`
- 代码表异步加载，启动后数秒内 `/codes` 为空、`name` 为空

## 开发

```bash
go test ./...
go test -tags live -run TestLiveProbe -v ./internal/protocol/   # 连真实服务器
```
