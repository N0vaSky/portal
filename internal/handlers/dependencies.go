package handlers

import (
	"github.com/N0vaSky/portal/internal/api/handlers/agent"
	"github.com/N0vaSky/portal/internal/api/handlers/alert"
	"github.com/N0vaSky/portal/internal/api/handlers/apikey"
	"github.com/N0vaSky/portal/internal/api/handlers/audit"
	"github.com/N0vaSky/portal/internal/api/handlers/auth"
	"github.com/N0vaSky/portal/internal/api/handlers/command"
	"github.com/N0vaSky/portal/internal/api/handlers/config"
	"github.com/N0vaSky/portal/internal/api/handlers/log"
	"github.com/N0vaSky/portal/internal/api/handlers/node"
	"github.com/N0vaSky/portal/internal/api/handlers/rule"
	"github.com/N0vaSky/portal/internal/api/handlers/user"
	"github.com/N0vaSky/portal/internal/config"
	"github.com/N0vaSky/portal/internal/services/alerts"
	"github.com/N0vaSky/portal/internal/services/auth/jwt"
	"github.com/N0vaSky/portal/internal/services/auth/mfa"
	"github.com/N0vaSky/portal/internal/services/configs"
	"github.com/N0vaSky/portal/internal/services/isolation"
	"github.com/N0vaSky/portal/internal/services/logs"
	"github.com/N0vaSky/portal/internal/services/nodes"
	"github.com/N0vaSky/portal/internal/services/remediation"
	"github.com/N0vaSky/portal/internal/services/rules"
	"github.com/N0vaSky/portal/internal/services/users"
	"github.com/jmoiron/sqlx"
)

// HandlerDependencies holds all the handlers used by the API
type HandlerDependencies struct {
	NodeHandler    *node.Handler
	RuleHandler    *rule.Handler
	AlertHandler   *alert.Handler
	LogHandler     *log.Handler
	CommandHandler *command.Handler
	ConfigHandler  *config.Handler
	UserHandler    *user.Handler
	AuthHandler    *auth.Handler
	APIKeyHandler  *apikey.Handler
	AuditHandler   *audit.Handler
	AgentHandler   *agent.Handler
}

// NewHandlerDependencies creates a new instance of HandlerDependencies
func NewHandlerDependencies(cfg *config.Config, db *sqlx.DB) *HandlerDependencies {
	// Initialize services
	nodeService := nodes.NewService(db)
	ruleService := rules.NewService(db, cfg.Fibratus.DefaultRulesDirPath)
	alertService := alerts.NewService(db, cfg.Fibratus.AlertsJsonPath)
	logService := logs.NewService(db)
	remediationService := remediation.NewService(db)
	configService := configs.NewService(db)
	userService := users.NewService(db)
	jwtService := jwt.NewService(cfg.Auth.JWTSecret)
	mfaService := mfa.NewService()
	isolationService := isolation.NewService(db)

	// Initialize handlers
	return &HandlerDependencies{
		NodeHandler:    node.NewHandler(nodeService, isolationService),
		RuleHandler:    rule.NewHandler(ruleService),
		AlertHandler:   alert.NewHandler(alertService),
		LogHandler:     log.NewHandler(logService),
		CommandHandler: command.NewHandler(remediationService),
		ConfigHandler:  config.NewHandler(configService),
		UserHandler:    user.NewHandler(userService),
		AuthHandler:    auth.NewHandler(userService, jwtService, mfaService, cfg),
		APIKeyHandler:  apikey.NewHandler(db),
		AuditHandler:   audit.NewHandler(db),
		AgentHandler:   agent.NewHandler(nodeService, ruleService, alertService, remediationService, configService),
	}
}