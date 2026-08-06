# txbuilder-tron

TRON 区块链交易构造 gRPC 服务，提供地址验证、余额检查、交易原始数据构造等 API。

![服务架构示意图](docs/images/txbuilder-tron-architecture.png)

## 环境要求

- Go 1.25+
- protoc（用于编译 proto 文件）

## 目录结构

```
txbuilder-tron/
├── cmd/txbuilder-tron/
│   └── main.go              # 程序入口，gRPC 服务启动
├── internal/
│   ├── config/              # TOML 配置加载 & 日志初始化
│   │   ├── config.go
│   │   └── config_test.go
│   ├── service/             # gRPC handler 实现
│   │   ├── service.go
│   │   └── service_test.go
│   └── tron/                # TRON 相关工具
│       ├── address.go       # Base58 编解码、地址校验
│       ├── address_test.go
│       └── rpc.go           # TRON 节点 HTTP JSON-RPC 客户端
├── api/
│   ├── txbuilder.proto      # gRPC 服务和消息定义
│   ├── txbuilder.pb.go     # 生成的 protobuf 代码
│   └── txbuilder_grpc.pb.go
├── config/
│   └── config.toml          # 默认配置文件
├── scripts/
│   └── gen.sh              # protobuf 代码生成脚本
└── .golangci.yml           # linter 配置
```



## gRPC 接口

服务在 `host:port`（默认 `127.0.0.1:50051`）上暴露 `TxBuilder` gRPC 服务。

### 1. VerifyAddress

验证 TRON 地址（Base58check 格式，以 `T` 开头）。

```protobuf
rpc VerifyAddress(VerifyAddressRequest) returns (VerifyAddressResponse);
```

**请求：**


| 字段       | 类型     | 说明        |
| -------- | ------ | --------- |
| trace_id | string | 追踪 ID（必填） |
| address  | string | TRON 地址   |


**响应：**


| 字段       | 类型   | 说明     |
| -------- | ---- | ------ |
| is_valid | bool | 地址是否合法 |




### 2. VerifyContractAddress

验证 TRON 合约地址。

```protobuf
rpc VerifyContractAddress(VerifyContractAddressRequest) returns (VerifyContractAddressResponse);
```

**请求：**


| 字段       | 类型     | 说明        |
| -------- | ------ | --------- |
| trace_id | string | 追踪 ID（必填） |
| address  | string | 合约地址      |


**响应：**


| 字段       | 类型   | 说明     |
| -------- | ---- | ------ |
| is_valid | bool | 地址是否合法 |




### 3. CheckSufficientBalance

检查账户余额是否足够发起转账。

```protobuf
rpc CheckSufficientBalance(CheckSufficientBalanceRequest) returns (CheckSufficientBalanceResponse);
```

**请求：**


| 字段            | 类型     | 说明                                     |
| ------------- | ------ | -------------------------------------- |
| trace_id      | string | 追踪 ID（必填）                              |
| chain_id      | string | 链 ID，如 "tron"                          |
| coin_id       | string | 币种 ID                                  |
| is_basic_coin | bool   | `true` 表示 TRX（原生币），`false` 表示 TRC20 代币 |
| from_address  | string | 发送方地址                                  |
| amount        | string | 转账金额（TRX 单位为 Sun，代币为最小单位）              |
| contract      | string | 代币合约地址（`is_basic_coin=false` 时必填）      |


**响应：**


| 字段            | 类型   | 说明     |
| ------------- | ---- | ------ |
| is_sufficient | bool | 余额是否充足 |




### 4. BuildSignRawData

构造待签名的交易原始数据。

```protobuf
rpc BuildSignRawData(BuildSignRawDataRequest) returns (BuildSignRawDataResponse);
```

**请求：**


| 字段            | 类型     | 说明                                |
| ------------- | ------ | --------------------------------- |
| biz_id        | string | 业务 ID（必填，最多 36 字符）                |
| chain_id      | string | 链 ID                              |
| coin_id       | string | 币种 ID                             |
| is_basic_coin | bool   | `true` 表示 TRX，`false` 表示 TRC20 代币 |
| coin_symbol   | string | 代币符号                              |
| from_address  | string | 发送方地址                             |
| to_address    | string | 接收方地址                             |
| amount        | string | 金额（必须是纯数字字符串）                     |
| contract      | string | 代币合约地址（`is_basic_coin=false` 时必填） |


**响应：**


| 字段  | 类型     | 说明                             |
| --- | ------ | ------------------------------ |
| msg | string | 原始数据的 SHA-256 哈希（十六进制字符串，用于签名） |




## 调用示例



### 使用 grpcurl

先启动服务，再用 grpcurl 调用：

```bash
# 启动服务
go run ./cmd/txbuilder-tron/main.go

# 验证地址
grpcurl -plaintext -d '{
  "trace_id": "test-001",
  "address": "TLa2f6VPqBkREki5JsqLkNqyjLRBaMFKDY"
}' localhost:50051 api.TxBuilder/VerifyAddress

# 检查余额（TRX）
grpcurl -plaintext -d '{
  "trace_id": "test-002",
  "chain_id": "tron",
  "coin_id": "trx",
  "is_basic_coin": true,
  "from_address": "TLa2f6VPqBkREki5JsqLkNqyjLRBaMFKDY",
  "amount": "1000000"
}' localhost:50051 api.TxBuilder/CheckSufficientBalance

# 构造 TRX 转账交易
grpcurl -plaintext -d '{
  "biz_id": "tx-001",
  "chain_id": "tron",
  "coin_id": "trx",
  "is_basic_coin": true,
  "coin_symbol": "TRX",
  "from_address": "TLa2f6VPqBkREki5JsqLkNqyjLRBaMFKDY",
  "to_address": "TN3W4H6rK4f4tRhqDaJf9u5Q5yT5Z3s8fB",
  "amount": "1000000"
}' localhost:50051 api.TxBuilder/BuildSignRawData
```



### Go 客户端示例

```go
package main

import (
	"context"
	"fmt"

	"github.com/koku-web3/txbuilder-tron/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := api.NewTxBuilderClient(conn)
	ctx := context.Background()

	// 验证地址
	resp, err := client.VerifyAddress(ctx, &api.VerifyAddressRequest{
		TraceId: "test-001",
		Address: "TLa2f6VPqBkREki5JsqLkNqyjLRBaMFKDY",
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("地址合法: %v\n", resp.IsValid)

	// 构造 TRX 转账
	txResp, err := client.BuildSignRawData(ctx, &api.BuildSignRawDataRequest{
		BizId:       "tx-001",
		ChainId:     "tron",
		CoinId:      "trx",
		IsBasicCoin: true,
		CoinSymbol:  "TRX",
		FromAddress: "TLa2f6VPqBkREki5JsqLkNqyjLRBaMFKDY",
		ToAddress:   "TN3W4H6rK4f4tRhqDaJf9u5Q5yT5Z3s8fB",
		Amount:      "1000000",
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("签名消息: %s\n", txResp.Msg)
}
```



## 开发



### 生成 protobuf 代码

```bash
./scripts/gen.sh
```

需要安装：`protoc`、`protoc-gen-go`、`protoc-gen-go-grpc`。

### 运行测试

```bash
go test ./...
```



### 代码检查

```bash
golangci-lint run ./...
```



## 许可证

见 [COPYING](./COPYING)。