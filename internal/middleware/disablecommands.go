package middleware

import (
	"github.com/sarulabs/di/v2"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/config"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/stringutil"
	"github.com/zekroTJA/shireikan"
	"github.com/zekrotja/ken"
)

type DisableCommandsMiddleware struct {
	cfg config.Provider
}

var (
	_ ken.MiddlewareBefore = (*DisableCommandsMiddleware)(nil)
	_ shireikan.Middleware = (*DisableCommandsMiddleware)(nil) // Deprecated
)

func NewDisableCommandsMiddleware(ctn di.Container) *DisableCommandsMiddleware {
	return &DisableCommandsMiddleware{
		cfg: ctn.Get(static.DiConfig).(config.Provider),
	}
}

func (m *DisableCommandsMiddleware) Before(ctx *ken.Ctx) (next bool, err error) {
	next = true

	if m.isDisabled(ctx.Command.Name()) {
		next = false
		err = ctx.RespondError("This command is disabled by config.", "")
	}

	return
}

// Deprecated
func (m *DisableCommandsMiddleware) Handle(
	cmd shireikan.Command,
	ctx shireikan.Context,
	layer shireikan.MiddlewareLayer,
) (next bool, err error) {
	next = true

	for _, invoke := range cmd.GetInvokes() {
		if m.isDisabled(invoke) {
			next = false
			_, err = ctx.ReplyEmbedError("This command is disabled by config.", "")
			break
		}
	}

	return
}

// Deprecated
func (m *DisableCommandsMiddleware) GetLayer() shireikan.MiddlewareLayer {
	return shireikan.LayerBeforeCommand
}

func (m *DisableCommandsMiddleware) isDisabled(invoke string) bool {
	disabledCmds := m.cfg.Config().Discord.DisabledCommands
	return len(disabledCmds) != 0 && stringutil.ContainsAny(invoke, disabledCmds)
}
