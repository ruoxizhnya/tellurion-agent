// Package aistore owns the `ai` PostgreSQL schema: its DDL, its migration
// entry, and (from QT-17 ② onward) the reads/writes of experiment-domain data.
//
// Ownership ruling: ADR-D10
// (tellurion:docs/design/qt-decoupling/target-module-topology.md §ADR-D10) moves
// experiment-domain data from core to this repository. The tables below are a
// verbatim port of the existing core definitions
// (tellurion:pkg/storage/postgres.go — experiments :424, research :361-398,
// factor_genes :930, strategy_genes :954); only the schema prefix changes:
//
//	public.experiments    → ai.experiments
//	public.factor_genes   → ai.factor_genes
//	public.strategy_genes → ai.strategy_genes
//	public.factor_hypothesis → ai.factor_hypothesis
//	research.profile      → ai.research_profile
//	research.conclusion   → ai.research_conclusion
//	research.question     → ai.research_question
//
// The core tables stay in place (read-only, no longer written since the QT-17 ④
// stop-write switch on 2026-10-24); this schema is additive and never drops or
// rewrites them.
//
// Table names and columns are copied from core, never inferred from Go type
// names: the drift test next to this file compares both sides and fails when
// they diverge.
package aistore

// Schema is the PostgreSQL schema this repository owns.
const Schema = "ai"

// Tables lists every table the `ai` schema owns, in creation order. It is the
// checked-in claim that the drift test compares against Statements() and against
// core's DDL.
var Tables = []string{
	"ai.experiments",
	"ai.research_profile",
	"ai.research_conclusion",
	"ai.research_question",
	"ai.factor_genes",
	"ai.strategy_genes",
	"ai.factor_hypothesis",
}

// statements is the execution truth: idempotent DDL, executed in order by
// Migrate. It follows core's mechanism (no migration table, no down, no
// execution record — see tellurion:docs/migrations/README.md for the trade-off),
// so every statement must be safe to run on an empty database and on an
// already-migrated one.
var statements = []string{
	`CREATE SCHEMA IF NOT EXISTS ai`,

	// experiments — core public.experiments (:424). parent_id self-reference is
	// rewritten to the ai schema; a cross-schema FK would pin the new table to
	// the table we intend to retire.
	`CREATE TABLE IF NOT EXISTS ai.experiments (
		id            BIGSERIAL PRIMARY KEY,
		run_id        VARCHAR(64)  NOT NULL,
		seq           INT          NOT NULL,
		parent_id     BIGINT       REFERENCES ai.experiments(id) ON DELETE SET NULL,
		hypothesis    TEXT,
		params        JSONB        NOT NULL DEFAULT '{}',
		dataset_split VARCHAR(16)  NOT NULL DEFAULT 'train',
		strategy_name VARCHAR(100),
		expression    TEXT,
		status        VARCHAR(16)  NOT NULL DEFAULT 'running',
		metrics       JSONB,
		verdict       JSONB,
		error_message TEXT,
		created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
		finished_at   TIMESTAMPTZ,
		UNIQUE (run_id, seq)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_experiments_run_seq ON ai.experiments(run_id, seq)`,

	// research domain — core research.profile/conclusion/question (:361-398).
	`CREATE TABLE IF NOT EXISTS ai.research_profile (
		ticker VARCHAR(20) PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		schema_version INT NOT NULL DEFAULT 1,
		source_file TEXT,
		source_mtime TIMESTAMPTZ,
		last_researched DATE,
		needs_review BOOLEAN NOT NULL DEFAULT FALSE,
		review_reason TEXT,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS ai.research_conclusion (
		ticker VARCHAR(20) NOT NULL REFERENCES ai.research_profile(ticker) ON DELETE CASCADE,
		conclusion_id VARCHAR(16) NOT NULL,
		title TEXT NOT NULL,
		body TEXT,
		as_of VARCHAR(16),
		confidence VARCHAR(8),
		status VARCHAR(16) NOT NULL DEFAULT 'active',
		citations JSONB NOT NULL DEFAULT '[]',
		evidence_pointer TEXT,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (ticker, conclusion_id)
	)`,
	`CREATE TABLE IF NOT EXISTS ai.research_question (
		ticker VARCHAR(20) NOT NULL REFERENCES ai.research_profile(ticker) ON DELETE CASCADE,
		question_id VARCHAR(16) NOT NULL,
		text TEXT NOT NULL,
		status VARCHAR(16) NOT NULL DEFAULT 'open',
		raised_at VARCHAR(16),
		PRIMARY KEY (ticker, question_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_research_conclusion_ticker ON ai.research_conclusion(ticker)`,
	`CREATE INDEX IF NOT EXISTS idx_research_conclusion_status ON ai.research_conclusion(status)`,
	`CREATE INDEX IF NOT EXISTS idx_research_question_ticker ON ai.research_question(ticker)`,
	`CREATE INDEX IF NOT EXISTS idx_research_question_status ON ai.research_question(status)`,
	`CREATE INDEX IF NOT EXISTS idx_research_profile_needs_review ON ai.research_profile(needs_review) WHERE needs_review = TRUE`,

	// Gene pools — core public.factor_genes (:930) / public.strategy_genes (:954).
	`CREATE TABLE IF NOT EXISTS ai.factor_genes (
		id          VARCHAR(50) PRIMARY KEY,
		name        VARCHAR(100) NOT NULL,
		category    VARCHAR(30) NOT NULL,
		formula     TEXT NOT NULL,
		description TEXT,
		rationale   TEXT,
		ic          DOUBLE PRECISION DEFAULT 0,
		ir          DOUBLE PRECISION DEFAULT 0,
		turnover    DOUBLE PRECISION DEFAULT 0,
		sharpe      DOUBLE PRECISION DEFAULT 0,
		fitness     DOUBLE PRECISION DEFAULT 0,
		generation  INTEGER DEFAULT 0,
		parent_ids  JSONB DEFAULT '[]',
		status      VARCHAR(20) DEFAULT 'pending',
		created_at  TIMESTAMPTZ DEFAULT NOW(),
		updated_at  TIMESTAMPTZ DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_factor_genes_category ON ai.factor_genes(category)`,
	`CREATE INDEX IF NOT EXISTS idx_factor_genes_status ON ai.factor_genes(status)`,
	`CREATE INDEX IF NOT EXISTS idx_factor_genes_fitness ON ai.factor_genes(fitness DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_factor_genes_cat_status ON ai.factor_genes(category, status)`,

	`CREATE TABLE IF NOT EXISTS ai.strategy_genes (
		id            VARCHAR(50) PRIMARY KEY,
		name          VARCHAR(100) NOT NULL,
		description   TEXT,
		strategy_type VARCHAR(30) NOT NULL,
		code          TEXT,
		params        JSONB DEFAULT '{}',
		factor_ids    JSONB DEFAULT '[]',
		parent_ids    JSONB DEFAULT '[]',
		total_return  DOUBLE PRECISION DEFAULT 0,
		sharpe        DOUBLE PRECISION DEFAULT 0,
		max_drawdown  DOUBLE PRECISION DEFAULT 0,
		win_rate      DOUBLE PRECISION DEFAULT 0,
		fitness       DOUBLE PRECISION DEFAULT 0,
		generation    INTEGER DEFAULT 0,
		status        VARCHAR(20) DEFAULT 'pending',
		created_at    TIMESTAMPTZ DEFAULT NOW(),
		updated_at    TIMESTAMPTZ DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_strategy_genes_type ON ai.strategy_genes(strategy_type)`,
	`CREATE INDEX IF NOT EXISTS idx_strategy_genes_status ON ai.strategy_genes(status)`,
	`CREATE INDEX IF NOT EXISTS idx_strategy_genes_fitness ON ai.strategy_genes(fitness DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_strategy_genes_type_status ON ai.strategy_genes(strategy_type, status)`,

	// factor_hypothesis — core public.factor_hypothesis (Migration 032, P2-3).
	// 因子的假设来源（「这个因子凭什么有效」）。factor_name 做主键、覆盖式
	// 更新的语义在 store 的 SQL 里，与 core 逐字同款。
	`CREATE TABLE IF NOT EXISTS ai.factor_hypothesis (
		factor_name  VARCHAR(32) PRIMARY KEY,
		source_kind  VARCHAR(24) NOT NULL,
		hypothesis   TEXT        NOT NULL,
		reference    TEXT,
		created_at   TIMESTAMPTZ DEFAULT NOW(),
		updated_at   TIMESTAMPTZ DEFAULT NOW()
	)`,
}

// Statements returns a copy of the DDL in execution order. Mutating the result
// cannot change what Migrate runs.
func Statements() []string {
	out := make([]string, len(statements))
	copy(out, statements)
	return out
}
