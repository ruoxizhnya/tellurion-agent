// Package adoption 把 ai.adoption_ledger 折算成可判定的校准指标。
//
// 口径写死在这里，读面与测试都不得自行解释（AGENT-INTERVENTION-ADOPTION-LEDGER）：
//
//   - 校准集 = 已回填结果（outcome_hit 非空）且 is_holdout = FALSE 的行。
//     holdout 行只进 holdout 报告，**永不**混进校准集 —— 否则就是拿同一批
//     数据既拟合又验收。
//   - 命中 = outcome_hit = TRUE；未回填（NULL）不计入任何分母，单列 pending。
//   - 校准误差 = |实际命中率 − 平均声称概率|（总体口径，不分箱）。
//   - Brier = mean((声称概率 − 命中指示)^2)，作为误差的平方损失补充。
package adoption

import "tellurion-agent/pkg/aistore"

// Report 是一份校准读面。
type Report struct {
	SampleSize       int           `json:"sample_size"`
	HitRate          float64       `json:"hit_rate"`
	ClaimedRate      float64       `json:"claimed_rate"`
	CalibrationError float64       `json:"calibration_error"`
	BrierScore       float64       `json:"brier_score"`
	Pending          int           `json:"pending"`
	Holdout          HoldoutReport `json:"holdout"`
}

// HoldoutReport 是留出对账集的独立结果。它不参与校准误差，只用来回答
// 「模型在没见过的那批提案上命中多少」。
type HoldoutReport struct {
	SampleSize int     `json:"sample_size"`
	HitRate    float64 `json:"hit_rate"`
}

// Compute 按包注释里的口径折算。空输入返回全零，不返回 NaN。
func Compute(entries []aistore.AdoptionDecision) Report {
	var report Report

	var calibration, holdout int
	var hits, holdoutHits int
	var claimedSum, brierSum float64

	for _, e := range entries {
		if e.OutcomeHit == nil {
			report.Pending++
			continue
		}
		hit := *e.OutcomeHit
		if e.IsHoldout {
			holdout++
			if hit {
				holdoutHits++
			}
			continue
		}

		calibration++
		if hit {
			hits++
		}
		claimedSum += e.ClaimedConfidence
		actual := 0.0
		if hit {
			actual = 1.0
		}
		delta := e.ClaimedConfidence - actual
		brierSum += delta * delta
	}

	report.SampleSize = calibration
	if calibration > 0 {
		report.HitRate = float64(hits) / float64(calibration)
		report.ClaimedRate = claimedSum / float64(calibration)
		report.CalibrationError = abs(report.HitRate - report.ClaimedRate)
		report.BrierScore = brierSum / float64(calibration)
	}

	report.Holdout.SampleSize = holdout
	if holdout > 0 {
		report.Holdout.HitRate = float64(holdoutHits) / float64(holdout)
	}
	return report
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
