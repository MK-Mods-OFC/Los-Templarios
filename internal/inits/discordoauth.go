package inits

import (
	"github.com/sarulabs/di/v2"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/config"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/webserver/auth"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/discordoauth/v2"
)

func InitDiscordOAuth(container di.Container) *discordoauth.DiscordOAuth {
	cfg := container.Get(static.DiConfig).(config.Provider)
	oauthHandler := container.Get(static.DiOAuthHandler).(auth.RequestHandler)

	return discordoauth.NewDiscordOAuth(
		cfg.Config().Discord.ClientID,
		cfg.Config().Discord.ClientSecret,
		cfg.Config().WebServer.PublicAddr+static.EndpointAuthCB,
		oauthHandler.LoginFailedHandler,
		oauthHandler.LoginSuccessHandler,
	)
}
