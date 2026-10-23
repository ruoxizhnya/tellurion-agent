package aistore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	kitdb "github.com/ruoxizhnya/tellurion-kit/db"
)

// Store 是 agent 自有的实验/研究/假设域存储（QT-17 ②）。它持有 ai schema
// 的连接池，是 S3 删 replace 的数据前置：agent 不再借 core 的
// *PostgresStore 读写这块数据。
//
// 与 core 的 NewPostgresStore 一样，NewStore 是**启动路径**：连接 + 迁移
// （幂等 DDL）一步到位；库侧测试若只想连不迁，直接用 NewStoreWithPool。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 连接 PostgreSQL 并执行 ai schema 的幂等迁移。
func NewStore(ctx context.Context, dsn string, opts kitdb.PoolOptions) (*Store, error) {
	pool, err := kitdb.Connect(ctx, dsn, opts)
	if err != nil {
		return nil, fmt.Errorf("aistore: connect: %w", err)
	}
	s := &Store{pool: pool}
	if err := Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("aistore: migrate: %w", err)
	}
	if err := s.SeedBuiltins(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("aistore: seed: %w", err)
	}
	return s, nil
}

// NewStoreWithPool 用现成连接池构造 Store（不迁移、不播种）——库侧测试
// 与迁移已由外层保证时使用。
func NewStoreWithPool(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// SeedBuiltins 在库里还没有记录时写入内置因子假设。启动路径调用；与 core
// 的种子逻辑同款（ON CONFLICT DO NOTHING，不覆盖人写的假设）。
func (s *Store) SeedBuiltins(ctx context.Context) error {
	_, err := s.SeedBuiltinFactorHypotheses(ctx)
	return err
}

// Close 释放连接池。
func (s *Store) Close() {
	s.pool.Close()
}
