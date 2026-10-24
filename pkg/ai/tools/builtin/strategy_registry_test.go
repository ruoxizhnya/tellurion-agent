package builtin

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tellurion-agent/pkg/ai/client"
	"tellurion-agent/pkg/ai/tools"
)

// fakeStrategyCatalog implements StrategyCatalog for tool-level tests.
// The HTTP shape itself is pinned in pkg/ai/client/strategy_client_test.go —
// here we only exercise the tools' handling of the returned catalog.
type fakeStrategyCatalog struct {
	infos []client.StrategyInfo
	err   error
	calls int
}

func (f *fakeStrategyCatalog) ListStrategies(context.Context) ([]client.StrategyInfo, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.infos, nil
}

func sampleCatalog() []client.StrategyInfo {
	return []client.StrategyInfo{
		{
			Name:        "momentum",
			Description: "Momentum strategy",
			Parameters: []client.StrategyParameter{
				{Name: "lookback", Type: "int", Default: float64(20)},
			},
		},
		{
			Name:        "dual_momentum",
			Description: "Dual momentum strategy",
		},
	}
}

// ─── StrategyListTool tests ───────────────────────────────────────────

func TestStrategyListTool_Name(t *testing.T) {
	tt := NewStrategyListTool(&fakeStrategyCatalog{})
	assert.Equal(t, "strategy.list", tt.Name())
}

func TestStrategyListTool_Description(t *testing.T) {
	tt := NewStrategyListTool(&fakeStrategyCatalog{})
	assert.NotEmpty(t, tt.Description())
	assert.Contains(t, tt.Description(), "strateg")
}

func TestStrategyListTool_Parameters(t *testing.T) {
	tt := NewStrategyListTool(&fakeStrategyCatalog{})
	// strategy.list takes no parameters.
	assert.Nil(t, tt.Parameters())
}

func TestStrategyListTool_OutputSchema(t *testing.T) {
	tt := NewStrategyListTool(&fakeStrategyCatalog{})
	schema := tt.OutputSchema()
	assert.Equal(t, "array", schema.Type)
	assert.NotEmpty(t, schema.Fields)
}

func TestStrategyListTool_Execute_ReturnsCatalog(t *testing.T) {
	catalog := &fakeStrategyCatalog{infos: sampleCatalog()}
	tt := NewStrategyListTool(catalog)

	result, err := tt.Execute(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, sampleCatalog(), result)
	assert.Equal(t, 1, catalog.calls)
}

func TestStrategyListTool_Execute_PropagatesError(t *testing.T) {
	catalogErr := errors.New("list_strategies returned status 500")
	tt := NewStrategyListTool(&fakeStrategyCatalog{err: catalogErr})

	_, err := tt.Execute(context.Background(), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, catalogErr)
}

// ─── StrategyGetTool tests ────────────────────────────────────────────

func TestStrategyGetTool_Name(t *testing.T) {
	tt := NewStrategyGetTool(&fakeStrategyCatalog{})
	assert.Equal(t, "strategy.get", tt.Name())
}

func TestStrategyGetTool_Description(t *testing.T) {
	tt := NewStrategyGetTool(&fakeStrategyCatalog{})
	assert.NotEmpty(t, tt.Description())
}

func TestStrategyGetTool_Parameters(t *testing.T) {
	tt := NewStrategyGetTool(&fakeStrategyCatalog{})
	params := tt.Parameters()
	require.Len(t, params, 1)
	assert.Equal(t, "name", params[0].Name)
	assert.True(t, params[0].Required)
}

func TestStrategyGetTool_OutputSchema(t *testing.T) {
	tt := NewStrategyGetTool(&fakeStrategyCatalog{})
	schema := tt.OutputSchema()
	assert.Equal(t, "object", schema.Type)
	assert.NotEmpty(t, schema.Fields)
}

func TestStrategyGetTool_Execute_FiltersFromList(t *testing.T) {
	catalog := &fakeStrategyCatalog{infos: sampleCatalog()}
	tt := NewStrategyGetTool(catalog)

	args := map[string]interface{}{"name": "dual_momentum"}
	result, err := tt.Execute(context.Background(), args)
	require.NoError(t, err)

	info, ok := result.(client.StrategyInfo)
	require.True(t, ok, "result should be client.StrategyInfo, got %T", result)
	assert.Equal(t, "dual_momentum", info.Name)
	assert.Equal(t, "Dual momentum strategy", info.Description)
	// 按 name 过滤走的是 list 结果，一次调用即可 —— 不再单查 /api/strategies/{id}
	// （那条路径只查 DB，内置策略会 404）。
	assert.Equal(t, 1, catalog.calls)
}

func TestStrategyGetTool_Execute_NotFound(t *testing.T) {
	tt := NewStrategyGetTool(&fakeStrategyCatalog{infos: sampleCatalog()})
	args := map[string]interface{}{"name": "nonexistent_strategy_xyz_123"}
	_, err := tt.Execute(context.Background(), args)
	require.Error(t, err)
	// Should NOT be ErrInvalidArgs — args were valid, strategy just doesn't exist.
	assert.False(t, errors.Is(err, tools.ErrInvalidArgs))
	assert.Contains(t, err.Error(), "strategy.get")
}

func TestStrategyGetTool_Execute_MissingName(t *testing.T) {
	catalog := &fakeStrategyCatalog{infos: sampleCatalog()}
	tt := NewStrategyGetTool(catalog)
	_, err := tt.Execute(context.Background(), map[string]interface{}{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
	assert.Contains(t, err.Error(), "name")
	// 参数非法时不该打下游。
	assert.Zero(t, catalog.calls)
}

func TestStrategyGetTool_Execute_EmptyName(t *testing.T) {
	tt := NewStrategyGetTool(&fakeStrategyCatalog{infos: sampleCatalog()})
	_, err := tt.Execute(context.Background(), map[string]interface{}{"name": ""})
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
}

func TestStrategyGetTool_Execute_WrongTypeName(t *testing.T) {
	tt := NewStrategyGetTool(&fakeStrategyCatalog{infos: sampleCatalog()})
	_, err := tt.Execute(context.Background(), map[string]interface{}{"name": 42})
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
}

func TestStrategyGetTool_Execute_PropagatesListError(t *testing.T) {
	catalogErr := errors.New("list_strategies returned status 401")
	tt := NewStrategyGetTool(&fakeStrategyCatalog{err: catalogErr})

	_, err := tt.Execute(context.Background(), map[string]interface{}{"name": "momentum"})
	require.Error(t, err)
	assert.ErrorIs(t, err, catalogErr)
	assert.Contains(t, err.Error(), "strategy.get")
}

// ─── Wiring / registration ────────────────────────────────────────────

func TestStrategyTools_PanicOnNilCatalog(t *testing.T) {
	assert.Panics(t, func() { NewStrategyListTool(nil) })
	assert.Panics(t, func() { NewStrategyGetTool(nil) })
}

func TestStrategyTools_RegisterInRegistry(t *testing.T) {
	catalog := &fakeStrategyCatalog{infos: sampleCatalog()}
	listTool := NewStrategyListTool(catalog)
	getTool := NewStrategyGetTool(catalog)

	reg := tools.NewRegistry()
	require.NoError(t, reg.Register(listTool))
	require.NoError(t, reg.Register(getTool))

	list := reg.List()
	require.Len(t, list, 2)
	// Sorted: strategy.get < strategy.list
	assert.Equal(t, "strategy.get", list[0].Name)
	assert.Equal(t, "strategy.list", list[1].Name)
}
