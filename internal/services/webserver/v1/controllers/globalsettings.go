package controllers

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/gofiber/fiber/v2"
	"github.com/sarulabs/di/v2"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/permissions"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/webserver/v1/models"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/presence"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/zekrotja/dgrs"
)

type GlobalSettingsController struct {
	session *discordgo.Session
	db      database.Database
	st      *dgrs.State
}

func (c *GlobalSettingsController) Setup(container di.Container, router fiber.Router) {
	c.session = container.Get(static.DiDiscordSession).(*discordgo.Session)
	c.db = container.Get(static.DiDatabase).(database.Database)
	c.st = container.Get(static.DiState).(*dgrs.State)

	pmw := container.Get(static.DiPermissions).(*permissions.Permissions)

	router.Get("/presence", pmw.HandleWs(c.session, "sp.presence"), c.getPresence)
	router.Post("/presence", pmw.HandleWs(c.session, "sp.presence"), c.postPresence)
	router.Get("/noguildinvite", pmw.HandleWs(c.session, "sp.noguildinvite"), c.getNoGuildInvites)
	router.Post("/noguildinvite", pmw.HandleWs(c.session, "sp.noguildinvite"), c.postNoGuildInvites)
}

// @Summary Get Presence
// @Description Returns the bot's displayed presence status.
// @Tags Global Settings
// @Accept json
// @Produce json
// @Success 200 {object} presence.Presence
// @Failure 401 {object} models.Error
// @Router /settings/presence [get]
func (c *GlobalSettingsController) getPresence(ctx *fiber.Ctx) error {
	presenceRaw, err := c.db.GetSetting(static.SettingPresence)
	if err != nil {
		if database.IsErrDatabaseNotFound(err) {
			return ctx.JSON(&presence.Presence{
				Game:   static.StdMotd,
				Status: "online",
			})
		}
		return err
	}

	pre, err := presence.Unmarshal(presenceRaw)
	if err != nil {
		return err
	}

	return ctx.JSON(pre)
}

// @Summary Set Presence
// @Description Set the bot's displayed presence status.
// @Tags Global Settings
// @Accept json
// @Produce json
// @Param payload body presence.Presence true "Presence Payload"
// @Success 200 {object} models.APITokenResponse
// @Failure 401 {object} models.Error
// @Failure 404 {object} models.Error "Is returned when no token was generated before."
// @Router /settings/presence [post]
func (c *GlobalSettingsController) postPresence(ctx *fiber.Ctx) error {
	pre := new(presence.Presence)
	if err := ctx.BodyParser(pre); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if err := pre.Validate(); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if err := c.db.SetSetting(static.SettingPresence, pre.Marshal()); err != nil {
		return err
	}

	if err := c.session.UpdateStatusComplex(pre.ToUpdateStatusData()); err != nil {
		return err
	}

	return ctx.JSON(pre)
}

// @Summary Get No Guild Invites Status
// @Description Returns the settings status for the suggested guild invite when the logged in user is not on any guild with shinpuru.
// @Tags Global Settings
// @Accept json
// @Produce json
// @Success 200 {object} models.InviteSettingsResponse
// @Failure 401 {object} models.Error
// @Failure 409 {object} models.Error "Returned when no channel could be found to create invite for."
// @Router /settings/noguildinvite [get]
func (c *GlobalSettingsController) getNoGuildInvites(ctx *fiber.Ctx) error {
	var guildID, message, inviteCode string
	var err error

	empty := func() error { return ctx.JSON(&models.InviteSettingsResponse{}) }

	if guildID, err = c.db.GetSetting(static.SettingWIInviteGuildID); err != nil {
		if err != nil && !database.IsErrDatabaseNotFound(err) {
			return err
		}
	}

	if guildID == "" {
		return empty()
	}

	if message, err = c.db.GetSetting(static.SettingWIInviteText); err != nil {
		if err != nil && !database.IsErrDatabaseNotFound(err) {
			return err
		}
	}

	if inviteCode, err = c.db.GetSetting(static.SettingWIInviteCode); err != nil {
		if err != nil && !database.IsErrDatabaseNotFound(err) {
			return err
		}
	}

	guild, err := c.st.Guild(guildID, true)
	if apiErr, ok := err.(*discordgo.RESTError); ok && apiErr.Message.Code == discordgo.ErrCodeMissingAccess {
		if err = c.db.SetSetting(static.SettingWIInviteGuildID, ""); err != nil {
			return err
		}
		return empty()
	}

	if err != nil {
		return err
	}

	invites, err := c.session.GuildInvites(guildID)
	if err != nil {
		return err
	}

	if inviteCode != "" {
		self, err := c.st.SelfUser()
		if err != nil {
			return err
		}
		for _, inv := range invites {
			if inv.Inviter != nil && inv.Inviter.ID == self.ID && !inv.Revoked {
				inviteCode = inv.Code
				break
			}
		}
	}

	if inviteCode == "" {
		chans, err := c.st.Channels(guild.ID)
		if err != nil {
			return err
		}
		var channel *discordgo.Channel
		for _, c := range chans {
			if c.Type == discordgo.ChannelTypeGuildText {
				channel = c
				break
			}
		}
		if channel == nil {
			return fiber.NewError(fiber.StatusConflict, "could not find any channel to create invite for")
		}

		invite, err := c.session.ChannelInviteCreate(channel.ID, discordgo.Invite{
			Temporary: false,
		})
		if err != nil {
			return err
		}

		inviteCode = invite.Code
		if err = c.db.SetSetting(static.SettingWIInviteCode, inviteCode); err != nil {
			return err
		}
	}

	res := &models.InviteSettingsResponse{
		Message:   message,
		InviteURL: fmt.Sprintf("https://discord.gg/%s", inviteCode),
	}

	res.Guild, err = models.GuildFromGuild(guild, nil, nil, "")
	if err != nil {
		return err
	}

	return ctx.JSON(res)
}

// @Summary Set No Guild Invites Status
// @Description Set the status for the suggested guild invite when the logged in user is not on any guild with shinpuru.
// @Tags Global Settings
// @Accept json
// @Produce json
// @Param payload body models.InviteSettingsRequest true "Invite Settings Payload"
// @Success 200 {object} models.APITokenResponse
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 409 {object} models.Error "Returned when no channel could be found to create invite for."
// @Router /settings/noguildinvite [post]
func (c *GlobalSettingsController) postNoGuildInvites(ctx *fiber.Ctx) error {
	req := new(models.InviteSettingsRequest)
	if err := ctx.BodyParser(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var err error

	if req.GuildID != "" {

		guild, err := c.st.Guild(req.GuildID, true)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}

		if req.InviteCode != "" {
			invites, err := c.session.GuildInvites(req.GuildID)
			if err != nil {
				return err
			}

			var valid bool
			for _, inv := range invites {
				if inv.Code == req.InviteCode && !inv.Revoked {
					valid = true
					break
				}
			}

			if !valid {
				return fiber.NewError(fiber.StatusBadRequest, "invalid invite code")
			}
		} else {
			var channel *discordgo.Channel
			chans, err := c.st.Channels(guild.ID, true)
			if err != nil {
				return err
			}
			for _, c := range chans {
				if c.Type == discordgo.ChannelTypeGuildText {
					channel = c
					break
				}
			}
			if channel == nil {
				return fiber.NewError(fiber.StatusConflict, "could not find any channel to create invite for")
			}

			invite, err := c.session.ChannelInviteCreate(channel.ID, discordgo.Invite{
				Temporary: false,
			})
			if err != nil {
				return err
			}

			req.InviteCode = invite.Code
		}
	}

	if err = c.db.SetSetting(static.SettingWIInviteCode, req.InviteCode); err != nil {
		return err
	}

	if err = c.db.SetSetting(static.SettingWIInviteGuildID, req.GuildID); err != nil {
		return err
	}

	if err = c.db.SetSetting(static.SettingWIInviteText, req.Messsage); err != nil {
		return err
	}

	return ctx.JSON(models.Ok)
}
