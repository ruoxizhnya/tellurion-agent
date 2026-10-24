package agents

import "tellurion-agent/pkg/ai/client"

// testCatalog 构造一份 core 目录快照夹具：行情有数据、财报表为空
// （与 OBS-08 实测的真库现状一致）。它只用于渲染语义测试；目录本身是否
// 与 core 注册表一致，由 env-gated 的对拍腿验证（builtin 包）。
func testCatalog() *client.ExpressionCatalog {
	return buildTestCatalog(false)
}

// allAvailableCatalog 是全部字段都「有数据」的对照组。
func allAvailableCatalog() *client.ExpressionCatalog {
	return buildTestCatalog(true)
}

func buildTestCatalog(fundamentalsAvailable bool) *client.ExpressionCatalog {
	field := func(name, source, status string) client.ExpressionField {
		return client.ExpressionField{Name: name, Source: source, Status: status}
	}
	fundStatus := "unavailable"
	if fundamentalsAvailable {
		fundStatus = "available"
	}
	return &client.ExpressionCatalog{
		Availability: "probed",
		Operators: []client.ExpressionOperator{
			{Name: "cs_neutralize", Category: "cs", Arity: 2},
			{Name: "cs_rank", Category: "cs", Arity: 1},
			{Name: "+", Category: "binary", Arity: 2},
			{Name: ">", Category: "binary", Arity: 2},
			{Name: "exp", Category: "unary", Arity: 1},
			{Name: "neg", Category: "unary", Arity: 1},
			{Name: "ts_mean", Category: "ts", Arity: 2},
			{Name: "ts_pct_change", Category: "ts", Arity: 2},
			{Name: "ts_std", Category: "ts", Arity: 2},
		},
		Fields: []client.ExpressionField{
			field("close", "market", "available"),
			field("high", "market", "available"),
			field("low", "market", "available"),
			field("open", "market", "available"),
			field("turnover", "market", "available"),
			field("volume", "market", "available"),
			field("turnover_rate", "activity", "available"),
			field("pe", "fundamentals", fundStatus),
			field("pb", "fundamentals", fundStatus),
			field("profit", "fundamentals", fundStatus),
			field("ps", "fundamentals", fundStatus),
			field("revenue", "fundamentals", fundStatus),
			field("roa", "fundamentals", fundStatus),
			field("roe", "fundamentals", fundStatus),
			field("sector", "group", fundStatus),
		},
	}
}
