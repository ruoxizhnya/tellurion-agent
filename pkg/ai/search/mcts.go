package search

import (
	"math"
	"math/rand"
)

// MCTSOptimizer 是参数空间上的确定性 UCT 树搜索（DBT-18）。
//
// 与 TPE 的差别：TPE 用核密度估计「好参数长什么样」；这里把每个参数切成
// buckets 个桶，把已观测尝试的 reward 记到对应的桶组合上，再用 UCB1 在树里
// 选择下一组桶，最后在选中的桶内均匀取一个点。
//
// 三条刻意的口径：
//   - **不读墙钟**。所有随机性来自构造时传入的 seed，同一个 seed + 同一串
//     观测必须给出同一串建议（回放与对比测试都靠它）。
//   - **不调模型**。它不产生 LLM 成本；将来若加 LLM 提议器，成本必须走
//     pkg/ai/cost.go 的 CostTable，不能在这里另起一套。
//   - **没有成功观测时不做伪拟合**。树没有信号，直接均匀采样——这和 TPE 的
//     startup 阶段是同一个理由：历史太少时硬拟合只会过拟合噪声。
type MCTSOptimizer struct {
	rng         *rand.Rand
	iterations  int
	exploration float64
	buckets     int
}

// NewMCTSOptimizer 构造一个确定性 MCTS。seed 相同、观测序列相同 ⇒ 建议序列相同。
func NewMCTSOptimizer(seed int64) *MCTSOptimizer {
	return &MCTSOptimizer{
		rng:         rand.New(rand.NewSource(seed)),
		iterations:  200,
		exploration: math.Sqrt2,
		buckets:     8,
	}
}

// mctsNode 是树里的一个桶前缀。children 按桶下标索引（不是 map），
// 这样遍历顺序天然确定，不受 Go map 随机序影响。
type mctsNode struct {
	depth     int
	visits    int
	rewardSum float64
	children  []*mctsNode
}

// Suggest 给出下一组待试参数（单步采样）。
//
// history 里的 Trial 由调用方过滤成「成功观测」；这里仍会再挡一次 State ==
// "failed"，因为失败尝试的 Value 是 0 而语义是「没跑出来」，喂进来会把分布
// 带偏。
func (o *MCTSOptimizer) Suggest(space *SearchSpace, history []*Trial) map[string]interface{} {
	if space == nil || len(space.Params) == 0 {
		return map[string]interface{}{}
	}

	usable := make([]*Trial, 0, len(history))
	for _, t := range history {
		if t == nil || t.State == "failed" {
			continue
		}
		usable = append(usable, t)
	}
	if len(usable) == 0 {
		return o.sampleUniform(space)
	}

	scale := newRewardScale(usable)
	root := &mctsNode{depth: 0, children: o.childrenFor(space, 0)}

	// 先用历史把树铺一遍：已知的桶组合带着实测 reward 进入 UCB，搜索不必
	// 从零再「发现」一次。
	for _, t := range usable {
		path := o.cellPath(space, t.Params)
		o.ensurePath(space, root, path)
		o.backprop(root, path, scale.reward(t.Value))
	}

	// 每轮：选择 → 扩展 → 用最近邻观测作为 rollout 的代理 reward → 回传。
	// 没有真实评估器时，这是「这一片区域历史上开出来的分」的保守代理。
	for i := 0; i < o.iterations; i++ {
		path := o.selectPath(space, root)
		full := o.completePath(space, path)
		reward := o.rolloutReward(space, full, usable, scale)
		o.ensurePath(space, root, path)
		o.backprop(root, path, reward)
	}

	return o.samplePath(space, o.bestPath(space, root))
}

// selectPath 从根走到叶子；遇到未访问的孩子先扩展它。
func (o *MCTSOptimizer) selectPath(space *SearchSpace, root *mctsNode) []int {
	path := make([]int, 0, len(space.Params))
	node := root
	for depth := 0; depth < len(space.Params); depth++ {
		idx := o.selectChild(node)
		if node.children[idx] == nil {
			node.children[idx] = &mctsNode{depth: depth + 1, children: o.childrenFor(space, depth+1)}
		}
		path = append(path, idx)
		node = node.children[idx]
	}
	return path
}

// selectChild 是 UCB1：未访问的孩子优先；都访问过就取
// mean + c*sqrt(ln(parentVisits)/visits)。
func (o *MCTSOptimizer) selectChild(n *mctsNode) int {
	unvisited := make([]int, 0, len(n.children))
	for i, c := range n.children {
		if c == nil || c.visits == 0 {
			unvisited = append(unvisited, i)
		}
	}
	if len(unvisited) > 0 {
		return unvisited[o.rng.Intn(len(unvisited))]
	}

	best, bestScore := 0, math.Inf(-1)
	logParent := math.Log(float64(n.visits) + 1)
	for i, c := range n.children {
		mean := c.rewardSum / float64(c.visits)
		score := mean + o.exploration*math.Sqrt(logParent/float64(c.visits))
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	return best
}

// bestPath 在收敛后的树上取「平均 reward 最高」的一支（平手看访问次数，
// 再平手取小下标——必须有一个确定的裁决，否则同 seed 不可复现）。
func (o *MCTSOptimizer) bestPath(space *SearchSpace, root *mctsNode) []int {
	path := make([]int, 0, len(space.Params))
	node := root
	for depth := 0; depth < len(space.Params); depth++ {
		best := -1
		bestMean := math.Inf(-1)
		bestVisits := -1
		for i, c := range node.children {
			if c == nil || c.visits == 0 {
				continue
			}
			mean := c.rewardSum / float64(c.visits)
			if mean > bestMean || (mean == bestMean && c.visits > bestVisits) {
				best, bestMean, bestVisits = i, mean, c.visits
			}
		}
		if best < 0 {
			// 树没有覆盖到这一支（理论上不会发生）；退回均匀采样，
			// 而不是给出一个越界或空参数。
			return o.sampleIndices(space)
		}
		path = append(path, best)
		node = node.children[best]
	}
	return path
}

// completePath 把选择阶段留下的部分路径补成完整路径。
func (o *MCTSOptimizer) completePath(space *SearchSpace, path []int) []int {
	full := append([]int(nil), path...)
	for depth := len(full); depth < len(space.Params); depth++ {
		full = append(full, o.rng.Intn(o.childCount(space.Params[depth])))
	}
	return full
}

// rolloutReward 用「最近邻观测」给一次 rollout 打分：在归一化参数距离下
// 找最接近的已知尝试，拿它的 reward。它是代理，不是真评估——真评估只有
// 底座回测能做。
func (o *MCTSOptimizer) rolloutReward(space *SearchSpace, cells []int, history []*Trial, scale rewardScale) float64 {
	bestDist := math.Inf(1)
	bestReward := 0.5
	for _, t := range history {
		d := o.cellDistance(space, cells, t.Params)
		if d < bestDist {
			bestDist = d
			bestReward = scale.reward(t.Value)
		}
	}
	return bestReward
}

func (o *MCTSOptimizer) cellDistance(space *SearchSpace, cells []int, params map[string]interface{}) float64 {
	var dist float64
	for i, d := range space.Params {
		if d.Type == "categorical" {
			if o.cellIndex(d, params[d.Name]) != cells[i] {
				dist++
			}
			continue
		}
		v, ok := toFloat(params[d.Name])
		if !ok {
			dist++
			continue
		}
		span := d.Max - d.Min
		if span <= 0 {
			continue
		}
		lo, hi := o.bucketBounds(d, cells[i])
		center := (lo + hi) / 2
		dist += math.Abs(v-center) / span
	}
	return dist
}

func (o *MCTSOptimizer) ensurePath(space *SearchSpace, root *mctsNode, path []int) {
	node := root
	for depth, idx := range path {
		if idx < 0 || idx >= len(node.children) {
			return
		}
		if node.children[idx] == nil {
			node.children[idx] = &mctsNode{depth: depth + 1, children: o.childrenFor(space, depth+1)}
		}
		node = node.children[idx]
	}
}

func (o *MCTSOptimizer) backprop(root *mctsNode, path []int, reward float64) {
	node := root
	node.visits++
	node.rewardSum += reward
	for _, idx := range path {
		if idx < 0 || idx >= len(node.children) || node.children[idx] == nil {
			return
		}
		node = node.children[idx]
		node.visits++
		node.rewardSum += reward
	}
}

func (o *MCTSOptimizer) childrenFor(space *SearchSpace, depth int) []*mctsNode {
	if depth >= len(space.Params) {
		return nil
	}
	return make([]*mctsNode, o.childCount(space.Params[depth]))
}

func (o *MCTSOptimizer) childCount(d ParamDef) int {
	if d.Type == "categorical" {
		if len(d.Choices) > 0 {
			return len(d.Choices)
		}
		return 1
	}
	return o.buckets
}

func (o *MCTSOptimizer) cellPath(space *SearchSpace, params map[string]interface{}) []int {
	path := make([]int, len(space.Params))
	for i, d := range space.Params {
		path[i] = o.cellIndex(d, params[d.Name])
	}
	return path
}

func (o *MCTSOptimizer) cellIndex(d ParamDef, v interface{}) int {
	if d.Type == "categorical" {
		s, ok := v.(string)
		if !ok {
			return 0
		}
		for i, c := range d.Choices {
			if c == s {
				return i
			}
		}
		return 0
	}

	f, ok := toFloat(v)
	if !ok {
		return 0
	}
	span := d.Max - d.Min
	if span <= 0 {
		return 0
	}
	b := int((f - d.Min) / span * float64(o.buckets))
	if b < 0 {
		return 0
	}
	if b >= o.buckets {
		return o.buckets - 1
	}
	return b
}

func (o *MCTSOptimizer) bucketBounds(d ParamDef, b int) (float64, float64) {
	span := d.Max - d.Min
	if span <= 0 {
		return d.Min, d.Max
	}
	if b < 0 {
		b = 0
	}
	if b >= o.buckets {
		b = o.buckets - 1
	}
	width := span / float64(o.buckets)
	lo := d.Min + width*float64(b)
	hi := lo + width
	if b == o.buckets-1 {
		hi = d.Max
	}
	return lo, hi
}

func (o *MCTSOptimizer) samplePath(space *SearchSpace, path []int) map[string]interface{} {
	out := make(map[string]interface{}, len(space.Params))
	for i, d := range space.Params {
		b := 0
		if i < len(path) {
			b = path[i]
		}
		switch d.Type {
		case "int":
			lo, hi := o.bucketBounds(d, b)
			loI := int(math.Ceil(lo))
			hiI := int(math.Floor(hi))
			if hiI < loI {
				hiI = loI
			}
			out[d.Name] = loI + o.rng.Intn(hiI-loI+1)
		case "categorical":
			if len(d.Choices) > 0 {
				if b >= len(d.Choices) {
					b = len(d.Choices) - 1
				}
				out[d.Name] = d.Choices[b]
			}
		default:
			lo, hi := o.bucketBounds(d, b)
			out[d.Name] = lo + o.rng.Float64()*(hi-lo)
		}
	}
	return out
}

func (o *MCTSOptimizer) sampleIndices(space *SearchSpace) []int {
	out := make([]int, len(space.Params))
	for i, d := range space.Params {
		out[i] = o.rng.Intn(o.childCount(d))
	}
	return out
}

func (o *MCTSOptimizer) sampleUniform(space *SearchSpace) map[string]interface{} {
	out := make(map[string]interface{}, len(space.Params))
	for _, d := range space.Params {
		switch d.Type {
		case "int":
			span := int(d.Max-d.Min) + 1
			if span < 1 {
				span = 1
			}
			out[d.Name] = int(d.Min) + o.rng.Intn(span)
		case "categorical":
			if len(d.Choices) > 0 {
				out[d.Name] = d.Choices[o.rng.Intn(len(d.Choices))]
			}
		default:
			out[d.Name] = d.Min + o.rng.Float64()*(d.Max-d.Min)
		}
	}
	return out
}

// rewardScale 把「越小越好」的目标值归一化成「越大越好」的 reward。
type rewardScale struct {
	min float64
	max float64
	ok  bool
}

func newRewardScale(trials []*Trial) rewardScale {
	s := rewardScale{min: math.Inf(1), max: math.Inf(-1)}
	for _, t := range trials {
		if t.Value < s.min {
			s.min = t.Value
		}
		if t.Value > s.max {
			s.max = t.Value
		}
		s.ok = true
	}
	return s
}

func (s rewardScale) reward(v float64) float64 {
	if !s.ok || s.max == s.min {
		return 0.5
	}
	return 1 - (v-s.min)/(s.max-s.min)
}

func toFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	default:
		return 0, false
	}
}
