package main

// ToolsHandler exposes the Tools Registry over HTTP (S7-P3-3, ODR-043).
//
// Endpoints:
//   GET  /api/tools          — list all tools with their schemas
//   GET  /api/tools/:name    — get a single tool's schema
//   POST /api/tools/:name    — execute a tool with args
//
// Design follows the "pattern B" handler style used by RiskHandler /
// ExecutionHandler / ComplianceHandler / PipelineHandler: a struct
// with functional options and a RegisterRoutes method. This is the
// project's演化方向 for new handlers.

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	kitauth "github.com/ruoxizhnya/tellurion-kit/auth"
	"tellurion-agent/pkg/ai/tools"
)

// 本仓的角色常量（QT-17 S3-b）。角色词表属于应用侧资产，kit/auth 只把
// 角色当不透明字符串处理；用户库与角色定义留在 core，本进程只消费
// token claims 里的 role 值。
const (
	RoleViewer = "viewer"
	RoleTrader = "trader"
	RoleAdmin  = "admin"
)

// ToolsHandler serves the /api/tools/* endpoints.
type ToolsHandler struct {
	registry *tools.Registry
	logger   zerolog.Logger

	// authSvc, when non-nil, enables per-tool RBAC on POST
	// /api/tools/:name. authMode is the process-wide 鉴权三态 fixed at
	// startup: enforced (RBAC), unavailable (503 auth_unavailable) or
	// open (explicitly declared insecure mode).
	//
	// AUD-02 (ODR-065 H5): this must be a *TokenService, not a bare
	// gin.HandlerFunc, because the role requirement depends on the
	// tool being called — and the tool name is only known inside the
	// handler (the route is /api/tools/:name, a wildcard, so no
	// per-route middleware can express it).
	authSvc  *kitauth.TokenService
	authMode authMode
}

// ToolsHandlerOption configures a ToolsHandler.
type ToolsHandlerOption func(*ToolsHandler)

// WithToolsLogger overrides the default logger.
func WithToolsLogger(l zerolog.Logger) ToolsHandlerOption {
	return func(h *ToolsHandler) { h.logger = l }
}

// WithToolsAuth enables per-tool RBAC using svc. POST /api/tools/:name
// requires a role derived from the tool's class:
//
//	read  -> viewer, trader, admin
//	write -> trader, admin
//	admin -> admin
//
// mode is the same 鉴权三态 the router uses (decideAuthStartup):
//
//   - authModeEnforced: RBAC applies (svc must be enabled).
//   - authModeUnavailable: every execution is 503 auth_unavailable —
//     there is no identity to authorize, so execution is not allowed.
//   - authModeOpen: explicitly declared insecure mode; no RBAC.
//
// The zero value (no option passed) is authModeUnavailable, i.e.
// fail-closed: constructing a handler without auth wiring never
// silently opens tool execution.
func WithToolsAuth(svc *kitauth.TokenService, mode authMode) ToolsHandlerOption {
	return func(h *ToolsHandler) {
		h.authSvc = svc
		h.authMode = mode
	}
}

// NewToolsHandler constructs a ToolsHandler backed by reg. Panics if
// reg is nil — wiring bug, fail loud at startup.
func NewToolsHandler(reg *tools.Registry, logger zerolog.Logger, opts ...ToolsHandlerOption) *ToolsHandler {
	if reg == nil {
		panic("analysis: NewToolsHandler called with nil Registry")
	}
	h := &ToolsHandler{
		registry: reg,
		logger:   logger.With().Str("component", "tools-handler").Logger(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RegisterRoutes mounts the /api/tools/* endpoints on the router.
func (h *ToolsHandler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api")
	tools := api.Group("/tools")
	{
		tools.GET("", h.handleList)
		tools.GET("/", h.handleList) // trailing-slash variant
		tools.GET("/:name", h.handleGet)
		tools.POST("/:name", h.handleExecute)
	}
}

// handleList returns all registered tools' ToolInfo (name + description
// + parameters + output_schema), sorted by name.
func (h *ToolsHandler) handleList(c *gin.Context) {
	list := h.registry.List()
	c.JSON(http.StatusOK, gin.H{
		"tools": list,
		"count": len(list),
	})
}

// handleGet returns a single tool's schema. Returns 404 if the tool
// name is not registered.
func (h *ToolsHandler) handleGet(c *gin.Context) {
	name := c.Param("name")
	t, err := h.registry.Get(name)
	if err != nil {
		h.respondToolError(c, http.StatusNotFound, err)
		return
	}

	info := tools.ToolInfo{
		Name:        t.Name(),
		Description: t.Description(),
	}
	if sp := tools.AsSchemaProvider(t); sp != nil {
		info.Parameters = sp.Parameters()
		info.OutputSchema = sp.OutputSchema()
	}
	c.JSON(http.StatusOK, info)
}

// executeRequest is the JSON body for POST /api/tools/:name.
type executeRequest struct {
	Args map[string]interface{} `json:"args"`
}

// executeResponse is the JSON body for a successful execution.
type executeResponse struct {
	Result interface{} `json:"result"`
}

// handleExecute looks up the tool by name and invokes Execute with the
// request body's "args" map.
func (h *ToolsHandler) handleExecute(c *gin.Context) {
	name := c.Param("name")

	// AUD-02 (ODR-065 H5): per-tool RBAC. The route is a wildcard
	// (/api/tools/:name), so the role requirement cannot be expressed
	// as per-route middleware — it is resolved here from the tool's
	// audited side-effect class.
	//
	// Checked BEFORE the Tool runs, and before body validation: an
	// unauthorized caller should not be able to distinguish "bad
	// request body" from "you may not call this" on a tool they
	// cannot call at all.
	if !h.authorizeTool(c, name) {
		return
	}

	var req executeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body: " + err.Error(),
			"code":  "INVALID_BODY",
		})
		return
	}

	result, err := h.registry.Execute(c.Request.Context(), name, req.Args)
	if err != nil {
		h.respondToolError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, executeResponse{Result: result})
}

// authorizeTool enforces the role requirement for the named tool.
// Returns true if the request may proceed; on false it has already
// written the error response.
//
// 鉴权三态在这里显式分流（与全局中间件同源，见 decideAuthStartup）：
//
//   - authModeOpen: 显式声明的放开形态，不做 RBAC；
//   - authModeUnavailable: 没有可用身份 → 503 auth_unavailable（fail-closed，
//     绝不 return true）；
//   - authModeEnforced: 按副作用等级做 RBAC；svc 缺失/未启用属于装配错误，
//     同样 503 auth_unavailable 而不是放行。
//
// Fail-closed: tools classified SideEffectAdmin (which includes every
// tool missing from the classification table) require admin.
func (h *ToolsHandler) authorizeTool(c *gin.Context, name string) bool {
	if h.authMode == authModeOpen {
		return true
	}
	if h.authMode != authModeEnforced || h.authSvc == nil || !h.authSvc.Enabled() {
		// 无 secret 且未声明放开，或 RBAC 装配缺失：没有可判定身份的闸门。
		// 写面与工具执行一律不可用，避免把「没配鉴权」静默变成「谁都能跑」。
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error": "authentication is not configured: tool execution is unavailable " +
				"until JWT_SECRET is set",
			"code": authUnavailableCode,
		})
		return false
	}

	var allowed []string
	switch tools.Classify(name) {
	case tools.SideEffectRead:
		allowed = []string{RoleViewer, RoleTrader, RoleAdmin}
	case tools.SideEffectWrite:
		allowed = []string{RoleTrader, RoleAdmin}
	default: // SideEffectAdmin
		allowed = []string{RoleAdmin}
	}

	role, ok := kitauth.RoleFromContext(c)
	if !ok {
		// No identity on the context. Either the global Middleware
		// didn't run or the request bypassed it; either way we cannot
		// authorize.
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "no authenticated user",
			"code":  "UNAUTHENTICATED",
		})
		return false
	}
	for _, r := range allowed {
		if r == role {
			return true
		}
	}

	h.logger.Warn().
		Str("tool", name).
		Str("effect", tools.Classify(name).String()).
		Str("have", role).
		Msg("tool execution denied by RBAC")

	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"error":  "insufficient role for tool " + name,
		"code":   "INSUFFICIENT_ROLE",
		"tool":   name,
		"effect": tools.Classify(name).String(),
		"have":   role,
		"need":   allowed,
	})
	return false
}

// respondToolError maps a tools error to an appropriate HTTP status
// code and a structured JSON error body. The error "code" field lets
// external agents distinguish failure modes programmatically.
func (h *ToolsHandler) respondToolError(c *gin.Context, status int, err error) {
	// Log the full error for diagnostics; the client gets a concise message.
	h.logger.Warn().Err(err).Str("tool", c.Param("name")).Msg("tool request failed")

	// Classify the error to pick the right status + code.
	switch {
	case errors.Is(err, tools.ErrToolNotRegistered):
		status = http.StatusNotFound
		c.JSON(status, gin.H{
			"error": err.Error(),
			"code":  "TOOL_NOT_REGISTERED",
		})
	case errors.Is(err, tools.ErrInvalidArgs):
		status = http.StatusBadRequest
		c.JSON(status, gin.H{
			"error": err.Error(),
			"code":  "INVALID_ARGS",
		})
	case errors.Is(err, tools.ErrNotFound):
		// Valid request, no such record (e.g. research.profile for a
		// ticker that was never archived). The message names the
		// missing entity — see research_tool.go's "no_profile".
		status = http.StatusNotFound
		c.JSON(status, gin.H{
			"error": err.Error(),
			"code":  "NOT_FOUND",
		})
	default:
		// Downstream / execution error — return 500 with the message
		// so the caller can see what went wrong (e.g. "strategy not
		// found: momentum"). We do NOT leak stack traces or internal
		// paths; the error string from the Tool is already
		// user-facing (it was constructed with fmt.Errorf in builtin/).
		c.JSON(status, gin.H{
			"error": err.Error(),
			"code":  "TOOL_EXECUTION_FAILED",
		})
	}
}
