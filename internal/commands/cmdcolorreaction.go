package commands

import (
	"strings"
	"time"

	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/zekroTJA/shireikan"
)

type CmdColorReaction struct {
}

func (c *CmdColorReaction) GetInvokes() []string {
	return []string{"color", "clr", "colorreaction"}
}

func (c *CmdColorReaction) GetDescription() string {
	return "Toggle color reactions enable or disable."
}

func (c *CmdColorReaction) GetHelp() string {
	return "`color` - toggle enable or disable\n" +
		"`color (enable|disable)` - set enabled or disabled"
}

func (c *CmdColorReaction) GetGroup() string {
	return shireikan.GroupGuildConfig
}

func (c *CmdColorReaction) GetDomainName() string {
	return "sp.guild.config.color"
}

func (c *CmdColorReaction) GetSubPermissionRules() []shireikan.SubPermission {
	return []shireikan.SubPermission{
		{
			Term:        "/sp.chat.colorreactions",
			Explicit:    false,
			Description: "Allows executing color reactions in chat by reaction",
		},
	}
}

func (c *CmdColorReaction) IsExecutableInDMChannels() bool {
	return false
}

func (c *CmdColorReaction) Exec(ctx shireikan.Context) (err error) {
	db, _ := ctx.GetObject(static.DiDatabase).(database.Database)

	var enabled bool

	if len(ctx.GetArgs()) == 0 {
		enabled, err = db.GetGuildColorReaction(ctx.GetGuild().ID)
		if err != nil {
			return
		}

		enabled = !enabled
	} else {
		switch strings.ToLower(ctx.GetArgs().Get(0).AsString()) {

		case "e", "enable", "enabled", "true", "on":
			enabled = true

		case "d", "disable", "disabled", "false", "off":
			enabled = false

		default:
			return util.SendEmbedError(ctx.GetSession(), ctx.GetChannel().ID,
				"Invalid argument. Use `help color` to see how to use this command.").
				Error()
		}
	}

	if err = db.SetGuildColorReaction(ctx.GetGuild().ID, enabled); err != nil {
		return
	}

	if enabled {
		return util.SendEmbed(ctx.GetSession(), ctx.GetChannel().ID,
			"Color reactions are now **enabled**.",
			"", static.ColorEmbedUpdated).
			DeleteAfter(8 * time.Second).Error()
	}

	return util.SendEmbed(ctx.GetSession(), ctx.GetChannel().ID,
		"Color reactions are now **disabled**.",
		"", static.ColorEmbedUpdated).
		DeleteAfter(8 * time.Second).Error()
}
