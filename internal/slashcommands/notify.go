package slashcommands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/permissions"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/acceptmsg"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/stringutil"
	"github.com/zekrotja/dgrs"
	"github.com/zekrotja/ken"
)

type Notify struct{}

var (
	_ ken.SlashCommand        = (*Notify)(nil)
	_ permissions.PermCommand = (*Notify)(nil)
)

func (c *Notify) Name() string {
	return "notify"
}

func (c *Notify) Description() string {
	return "Get, remove or setup the notify role."
}

func (c *Notify) Version() string {
	return "1.0.0"
}

func (c *Notify) Type() discordgo.ApplicationCommandType {
	return discordgo.ChatApplicationCommand
}

func (c *Notify) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "toggle",
			Description: "Get or remove notify role.",
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "setup",
			Description: "Setup notify role.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionRole,
					Name:        "role",
					Description: "The role to be used as notify role (will be created if not specified).",
				},
			},
		},
	}
}

func (c *Notify) Domain() string {
	return "sp.chat.notify"
}

func (c *Notify) SubDomains() []permissions.SubPermission {
	return []permissions.SubPermission{
		{
			Term:        "setup",
			Explicit:    true,
			Description: "Allows setting up the notify role for this guild.",
		},
	}
}

func (c *Notify) Run(ctx *ken.Ctx) (err error) {
	if err = ctx.Defer(); err != nil {
		return
	}

	err = ctx.HandleSubCommands(
		ken.SubCommandHandler{"toggle", c.toggle},
		ken.SubCommandHandler{"setup", c.setup},
	)

	return
}

func (c *Notify) toggle(ctx *ken.SubCommandCtx) (err error) {
	db := ctx.Get(static.DiDatabase).(database.Database)
	st := ctx.Get(static.DiState).(*dgrs.State)

	notifyRoleID, err := db.GetGuildNotifyRole(ctx.Event.GuildID)
	if database.IsErrDatabaseNotFound(err) || notifyRoleID == "" {
		return ctx.FollowUpError(
			"No notify role  was set up for this guild.", "").Error
	}
	if err != nil {
		return err
	}

	roles, err := st.Roles(ctx.Event.GuildID, true)
	if err != nil {
		return
	}
	var roleExists bool
	for _, role := range roles {
		if notifyRoleID == role.ID && !roleExists {
			roleExists = true
		}
	}

	if !roleExists {
		return ctx.FollowUpError(
			"The set notify role does not exist on this guild anymore. Please notify a "+
				"moderator aor admin about this to fix this. ;)", "").Error
	}

	member, err := st.Member(ctx.Event.GuildID, ctx.User().ID)
	if err != nil {
		return err
	}

	msgStr := "Removed notify role."
	if stringutil.IndexOf(notifyRoleID, member.Roles) > -1 {
		err = ctx.Session.GuildMemberRoleRemove(ctx.Event.GuildID, ctx.User().ID, notifyRoleID)
		if err != nil {
			return err
		}
	} else {
		err = ctx.Session.GuildMemberRoleAdd(ctx.Event.GuildID, ctx.User().ID, notifyRoleID)
		if err != nil {
			return err
		}
		msgStr = "Added notify role."
	}

	return ctx.FollowUpEmbed(&discordgo.MessageEmbed{
		Description: msgStr,
	}).Error
}

func (c *Notify) setup(ctx *ken.SubCommandCtx) (err error) {
	pmw := ctx.Get(static.DiPermissions).(*permissions.Permissions)
	db := ctx.Get(static.DiDatabase).(database.Database)
	st := ctx.Get(static.DiState).(*dgrs.State)

	ok, err := pmw.CheckSubPerm(ctx.Ctx, "setup", true)
	if err != nil {
		return err
	}
	if !ok {
		return ctx.FollowUpError(
			"Sorry, but you do'nt have the permission to setup the notify role.", "").
			Error
	}

	roles, err := st.Roles(ctx.Event.GuildID)
	var notifyRoleExists bool
	notifyRoleID, err := db.GetGuildNotifyRole(ctx.Event.GuildID)
	if err == nil {
		for _, role := range roles {
			if notifyRoleID == role.ID && !notifyRoleExists {
				notifyRoleExists = true
			}
		}
	}
	notifiableStr := "\n*Notify role is defaulty not notifiable. You need to enable this manually by using the " +
		"`ment` command or toggling it manually in the discord settings.*"
	if notifyRoleExists {
		am := &acceptmsg.AcceptMessage{
			Session:        ctx.Session,
			UserID:         ctx.User().ID,
			DeleteMsgAfter: true,
			Embed: &discordgo.MessageEmbed{
				Color: static.ColorEmbedDefault,
				Description: fmt.Sprintf("The notify role on this guild is already set to <@&%s>.\n"+
					"Do you want to overwrite this setting? This will also **delete** the role <@&%s>.",
					notifyRoleID, notifyRoleID),
			},
			AcceptFunc: func(m *discordgo.Message) (err error) {
				role, err := c.setupRole(ctx)
				if err != nil {
					return
				}
				err = ctx.Session.GuildRoleDelete(ctx.Event.GuildID, notifyRoleID)
				if err != nil {
					return
				}
				err = ctx.FollowUpEmbed(&discordgo.MessageEmbed{
					Description: fmt.Sprintf("Updated notify role to <@&%s>."+notifiableStr, role.ID),
				}).Error
				return
			},
			DeclineFunc: func(m *discordgo.Message) (err error) {
				err = ctx.FollowUpEmbed(&discordgo.MessageEmbed{
					Description: "Canceled",
				}).Error
				return
			},
		}

		if _, err := am.AsFollowUp(ctx.Ctx); err != nil {
			return err
		}

		return am.Error()
	}

	role, err := c.setupRole(ctx)
	if err != nil {
		return err
	}
	err = ctx.FollowUpEmbed(&discordgo.MessageEmbed{
		Description: fmt.Sprintf("Updated notify role to <@&%s>."+notifiableStr, role.ID),
	}).Error
	return
}

func (c *Notify) setupRole(ctx *ken.SubCommandCtx) (role *discordgo.Role, err error) {
	db, _ := ctx.Get(static.DiDatabase).(database.Database)

	const roleName = "Notify"
	if roleV, ok := ctx.Options().GetByNameOptional("role"); ok {
		role = roleV.RoleValue(ctx.Ctx)
	} else {
		role, err = ctx.Session.GuildRoleCreate(ctx.Event.GuildID)
		if err != nil {
			return
		}
		role, err = ctx.Session.GuildRoleEdit(ctx.Event.GuildID, role.ID, roleName, 0, false, 0, false)
		if err != nil {
			return nil, err
		}
	}

	err = db.SetGuildNotifyRole(ctx.Event.GuildID, role.ID)
	return role, err
}
