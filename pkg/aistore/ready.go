package aistore

import (
	"context"
	"fmt"
)

// requiredRelations 是 ai-service 就绪所需的最小关系集合（schema.table），
// 全部由启动迁移（Migrate）产出。判据与 core 的 storage.Ready 同款：
// 池能 ping 通**且**这些表在 —— 空库能 ping 通但一条 AI 请求都服务不了。
var requiredRelations = []string{
	"ai.experiments",
	"ai.factor_genes",
	"ai.strategy_genes",
	"ai.research_profile",
}

// Ready 报告本进程是否可以接流量（连接可用 + 启动迁移的关键表在位）。
// 调用方应自带超时（探针 2s 量级），本方法不再叠加。
func (s *Store) Ready(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("aistore: connection pool is not configured")
	}
	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("aistore: ping failed: %w", err)
	}
	for _, rel := range requiredRelations {
		var exists bool
		if err := s.pool.QueryRow(ctx,
			"SELECT to_regclass($1) IS NOT NULL", rel).Scan(&exists); err != nil {
			return fmt.Errorf("aistore: schema check %s failed: %w", rel, err)
		}
		if !exists {
			return fmt.Errorf("aistore: required relation %s is missing "+
				"(startup migration did not run or was rolled back)", rel)
		}
	}
	return nil
}
