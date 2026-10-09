package tron

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	//nolint:staticcheck // gotron-sdk uses github.com/golang/protobuf; must use same package to unmarshal its types.
	proto "github.com/golang/protobuf/proto"
)

// mockFetcher 用固定的区块响应实现 RefBlockFetcher。
type mockFetcher struct {
	response json.RawMessage
	err      error
}

func (m *mockFetcher) GetNowBlock(ctx context.Context) (json.RawMessage, error) {
	return m.response, m.err
}

// 通过 gotron-sdk 的 DecodeCheck 的有效 TRON 主网地址。
const validTestFrom = "TRGhNNfnmgLegT4zHNjEqDSADjgmnHvubJ"
const validTestTo = "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9"
const validTestContract = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"

// blockRespNum86945984: 32 个零字节（64 个十六进制字符）+ block_num=86945984 (0x052eb0c0)。
// 当在 block_num=86945984 查询时，节点产生这个 blockID：
//
//	refBlockBytes = 大端序(num)[6:8] = 0xb0c0
//	refBlockHash = blockID 字节[8:16] = cb95fd971bc93fc2
//
// 这已通过实时 TronGrid API 响应验证。
const blockRespNum86945984 = `{"blockID":"0x00000000052eb0c0cb95fd971bc93fc21683e18bb3f84b6427b0f32b57ae6ab7","block_header":{"raw_data":{"number":86945984}}}`

// blockRespNum1: 31 个零字节 + "01" (block_num=1)。
// refBlockBytes = 大端序字节[6:8] = {0x00, 0x01}
// refBlockHash = 字节[8:16]
const blockRespNum1 = `{"blockID":"0x0000000000000000000000000000000000000000000000000000000000000001","block_header":{"raw_data":{"number":1}}}`

func TestParseAmountInt64(t *testing.T) {
	tests := []struct {
		name    string
		amount  string
		want    int64
		wantErr bool
	}{
		{name: "valid small number", amount: "1000000", want: 1_000_000},
		{name: "valid zero", amount: "0", want: 0},
		{name: "valid max int64", amount: "9223372036854775807", want: 9223372036854775807},
		{name: "invalid format", amount: "abc", wantErr: true},
		{name: "negative number", amount: "-100", wantErr: true},
		{name: "decimal number", amount: "100.5", wantErr: true},
		{name: "empty string", amount: "", wantErr: true},
		{name: "overflow int64", amount: "9223372036854775808", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAmountInt64(tt.amount)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAmountInt64(%q) error = %v, wantErr %v", tt.amount, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseAmountInt64(%q) = %d, want %d", tt.amount, got, tt.want)
			}
		})
	}
}

func TestFetchRefBlock(t *testing.T) {
	fetcher := &mockFetcher{response: json.RawMessage(blockRespNum86945984)}

	ref, err := FetchRefBlock(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("FetchRefBlock() error = %v", err)
	}

	if ref.BlockNum != 86945984 {
		t.Errorf("BlockNum = %d, want 86945984", ref.BlockNum)
	}

	// block_num = 86945984 = 0x052EB0C0; 大端序字节[6:8] = {0xB0, 0xC0}
	if len(ref.RefBlockBytes) != 2 {
		t.Fatalf("RefBlockBytes length = %d, want 2", len(ref.RefBlockBytes))
	}
	if ref.RefBlockBytes[0] != 0xB0 || ref.RefBlockBytes[1] != 0xC0 {
		t.Errorf("RefBlockBytes = %x, want b0c0", ref.RefBlockBytes)
	}

	// refBlockHash = blockID 字节[8:16] = cb95fd971bc93fc2
	wantHash := "cb95fd971bc93fc2"
	if hex.EncodeToString(ref.RefBlockHash) != wantHash {
		t.Errorf("RefBlockHash = %s, want %s", hex.EncodeToString(ref.RefBlockHash), wantHash)
	}
}

func TestFetchRefBlock_Errors(t *testing.T) {
	t.Run("GetNowBlock returns error", func(t *testing.T) {
		fetcher := &mockFetcher{err: context.DeadlineExceeded}
		_, err := FetchRefBlock(context.Background(), fetcher)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("empty blockID", func(t *testing.T) {
		fetcher := &mockFetcher{response: json.RawMessage(`{"blockID":"","block_header":{"raw_data":{"number":1}}}`)}
		_, err := FetchRefBlock(context.Background(), fetcher)
		if err == nil {
			t.Error("expected error for empty blockID, got nil")
		}
	})

	t.Run("invalid blockID hex", func(t *testing.T) {
		fetcher := &mockFetcher{response: json.RawMessage(`{"blockID":"0xZZZZ","block_header":{"raw_data":{"number":1}}}`)}
		_, err := FetchRefBlock(context.Background(), fetcher)
		if err == nil {
			t.Error("expected error for invalid hex, got nil")
		}
	})
}

func TestBuildBasicCoinRawData_Structure(t *testing.T) {
	fetcher := &mockFetcher{response: json.RawMessage(blockRespNum86945984)}

	rawData, err := BuildBasicCoinRawData(context.Background(), fetcher, validTestFrom, validTestTo, "1000000")
	if err != nil {
		t.Fatalf("BuildBasicCoinRawData() error = %v", err)
	}
	if len(rawData) == 0 {
		t.Fatal("BuildBasicCoinRawData() returned empty rawData")
	}

	// 使用 decodeRawDataToJSON 验证
	jsonData, err := decodeRawDataToJSON(rawData)
	if err != nil {
		t.Fatalf("decodeRawDataToJSON() error = %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// ref_block_bytes = b0c0 (来自 block_num=86945984)
	refBlockBytesHex, ok := parsed["ref_block_bytes"].(string)
	if !ok {
		t.Fatal("ref_block_bytes not found")
	}
	rb, err := hex.DecodeString(refBlockBytesHex)
	if err != nil {
		t.Fatalf("hex.DecodeString(ref_block_bytes) error = %v", err)
	}
	if len(rb) != 2 {
		t.Fatalf("ref_block_bytes length = %d, want 2", len(rb))
	}
	if rb[0] != 0xB0 || rb[1] != 0xC0 {
		t.Errorf("ref_block_bytes = %x, want b0c0", rb)
	}

	// ref_block_hash = cb95fd971bc93fc2
	refBlockHashHex, ok := parsed["ref_block_hash"].(string)
	if !ok {
		t.Fatal("ref_block_hash not found")
	}
	if refBlockHashHex != "cb95fd971bc93fc2" {
		t.Errorf("ref_block_hash = %s, want cb95fd971bc93fc2", refBlockHashHex)
	}

	// 合约类型 = TransferContract（通过下面的 proto 验证）
	contracts, ok := parsed["contract"].([]interface{})
	if !ok || len(contracts) == 0 {
		t.Fatal("contract not found or empty")
	}

	// 到期时间 - 时间戳 = 5 分钟
	expiration, ok := parsed["expiration"].(float64)
	timestamp, hasTs := parsed["timestamp"].(float64)
	if !ok || !hasTs {
		t.Fatal("expiration or timestamp not found")
	}
	offset := int64(expiration) - int64(timestamp)
	if offset != ExpirationDuration {
		t.Errorf("expiration - timestamp = %d, want %d (5 minutes)", offset, ExpirationDuration)
	}

	// Proto 级别检查
	raw := &core.TransactionRaw{}
	if err := proto.Unmarshal(rawData, raw); err != nil {
		t.Fatalf("proto.Unmarshal to TransactionRaw error = %v", err)
	}
	if len(raw.Contract) != 1 {
		t.Fatalf("raw.Contract length = %d, want 1", len(raw.Contract))
	}
	if raw.Contract[0].Type != core.Transaction_Contract_TransferContract {
		t.Errorf("Contract type = %v, want TransferContract", raw.Contract[0].Type)
	}
	if raw.FeeLimit != 0 {
		t.Errorf("FeeLimit = %d, want 0 for basic coin", raw.FeeLimit)
	}

	t.Logf("raw_data_length=%d, ref_block_bytes=%s, contract_type=%v, expiration_offset=%dms",
		len(rawData), refBlockBytesHex, raw.Contract[0].Type, offset)
}

func TestBuildTokenRawData_Structure(t *testing.T) {
	fetcher := &mockFetcher{response: json.RawMessage(blockRespNum86945984)}

	feeLimit := int64(30_000_000)
	rawData, err := BuildTokenRawData(context.Background(), fetcher, validTestFrom, validTestTo, "1000000", validTestContract, feeLimit)
	if err != nil {
		t.Fatalf("BuildTokenRawData() error = %v", err)
	}

	// Proto 级别检查 — 比 JSON 字段名更可靠
	raw := &core.TransactionRaw{}
	if err := proto.Unmarshal(rawData, raw); err != nil {
		t.Fatalf("proto.Unmarshal to TransactionRaw error = %v", err)
	}
	if raw.Contract[0].Type != core.Transaction_Contract_TriggerSmartContract {
		t.Errorf("Contract type = %v, want TriggerSmartContract", raw.Contract[0].Type)
	}
	if raw.FeeLimit != feeLimit {
		t.Errorf("FeeLimit = %d, want %d", raw.FeeLimit, feeLimit)
	}

	// 调用数据：选择器(4 字节) + 填充地址(32 字节) + 填充金额(32 字节) = 68 字节
	tsc := &core.TriggerSmartContract{}
	if err := proto.Unmarshal(raw.Contract[0].GetParameter().GetValue(), tsc); err != nil {
		t.Fatalf("proto.Unmarshal to TriggerSmartContract error = %v", err)
	}
	if len(tsc.Data) != 68 {
		t.Errorf("calldata length = %d, want 68; calldata_hex = %s", len(tsc.Data), hex.EncodeToString(tsc.Data))
	}
	// 前 4 字节 = TRC20 转账选择器 0xa9059cbb
	if !bytes.HasPrefix(tsc.Data, []byte{0xa9, 0x05, 0x9c, 0xbb}) {
		t.Errorf("calldata selector = %x, want a9059cbb", tsc.Data[:4])
	}

	// 到期时间 - 时间戳 = 5 分钟
	jsonData, err := decodeRawDataToJSON(rawData)
	if err != nil {
		t.Fatalf("decodeRawDataToJSON() error = %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	expiration := parsed["expiration"].(float64)
	timestamp := parsed["timestamp"].(float64)
	offset := int64(expiration) - int64(timestamp)
	if offset != ExpirationDuration {
		t.Errorf("expiration - timestamp = %d, want %d (5 minutes)", offset, ExpirationDuration)
	}

	t.Logf("raw_data_length=%d, contract_type=%v, fee_limit=%d, calldata_length=%d, expiration_offset=%dms",
		len(rawData), raw.Contract[0].Type, raw.FeeLimit, len(tsc.Data), offset)
}

func TestBuildBasicCoinRawData_InvalidInput(t *testing.T) {
	fetcher := &mockFetcher{response: json.RawMessage(blockRespNum1)}

	tests := []struct {
		name   string
		from   string
		to     string
		amount string
	}{
		{name: "invalid from address", from: "invalid", to: validTestTo, amount: "1000000"},
		{name: "invalid to address", from: validTestFrom, to: "invalid", amount: "1000000"},
		{name: "invalid amount", from: validTestFrom, to: validTestTo, amount: "abc"},
		{name: "empty amount", from: validTestFrom, to: validTestTo, amount: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildBasicCoinRawData(context.Background(), fetcher, tt.from, tt.to, tt.amount)
			if err == nil {
				t.Errorf("BuildBasicCoinRawData() expected error, got nil")
			}
		})
	}
}

func TestBuildTokenRawData_InvalidInput(t *testing.T) {
	fetcher := &mockFetcher{response: json.RawMessage(blockRespNum1)}

	tests := []struct {
		name     string
		from     string
		to       string
		amount   string
		contract string
	}{
		{name: "invalid from address", from: "invalid", to: validTestTo, amount: "1000000", contract: validTestContract},
		{name: "invalid to address", from: validTestFrom, to: "invalid", amount: "1000000", contract: validTestContract},
		{name: "invalid contract", from: validTestFrom, to: validTestTo, amount: "1000000", contract: "invalid"},
		{name: "invalid amount", from: validTestFrom, to: validTestTo, amount: "abc", contract: validTestContract},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildTokenRawData(context.Background(), fetcher, tt.from, tt.to, tt.amount, tt.contract, 30_000_000)
			if err == nil {
				t.Errorf("BuildTokenRawData() expected error, got nil")
			}
		})
	}
}

func TestExpirationDuration_5Minutes(t *testing.T) {
	if ExpirationDuration != 5*60*1000 {
		t.Errorf("ExpirationDuration = %d, want %d", ExpirationDuration, 5*60*1000)
	}
}

func TestDefaultTRC20FeeLimit(t *testing.T) {
	if DefaultTRC20FeeLimit != 30_000_000 {
		t.Errorf("DefaultTRC20FeeLimit = %d, want 30_000_000", DefaultTRC20FeeLimit)
	}
}

func TestRefBlockInfo_Fields(t *testing.T) {
	fetcher := &mockFetcher{response: json.RawMessage(blockRespNum1)}

	ref, err := FetchRefBlock(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("FetchRefBlock() error = %v", err)
	}

	if len(ref.RefBlockBytes) != 2 {
		t.Errorf("RefBlockBytes length = %d, want 2", len(ref.RefBlockBytes))
	}
	if len(ref.RefBlockHash) != 8 {
		t.Errorf("RefBlockHash length = %d, want 8", len(ref.RefBlockHash))
	}
	if ref.BlockNum != 1 {
		t.Errorf("BlockNum = %d, want 1", ref.BlockNum)
	}
}

func TestBuildBasicCoinRawData_TimestampRecent(t *testing.T) {
	fetcher := &mockFetcher{response: json.RawMessage(blockRespNum1)}

	rawData, err := BuildBasicCoinRawData(context.Background(), fetcher, validTestFrom, validTestTo, "1000000")
	if err != nil {
		t.Fatalf("BuildBasicCoinRawData() error = %v", err)
	}

	raw := &core.TransactionRaw{}
	if err := proto.Unmarshal(rawData, raw); err != nil {
		t.Fatalf("proto.Unmarshal error = %v", err)
	}

	now := time.Now().UnixMilli()
	if raw.Timestamp < now-60_000 || raw.Timestamp > now+60_000 {
		t.Errorf("Timestamp = %d, expected near %d (±60s)", raw.Timestamp, now)
	}
}
