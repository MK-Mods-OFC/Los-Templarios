package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/bwmarrin/discordgo"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/fetch"
	"github.com/zekroTJA/shireikan"
	"github.com/zekrotja/dgrs"
)

const allowMask = discordgo.PermissionAll - discordgo.PermissionSendMessages

type CmdLock struct {
}

func (c *CmdLock) GetInvokes() []string {
	return []string{"lock", "unlock", "lockchan", "unlockchan", "readonly", "ro", "chatlock"}
}

func (c *CmdLock) GetDescription() string {
	return "Locks the channel so that no one can write there anymore until unlocked."
}

func (c *CmdLock) GetHelp() string {
	return "`lock (<channelResolvable>)` - locks or unlocks either the current or the passed channel\n"
}

func (c *CmdLock) GetGroup() string {
	return shireikan.GroupModeration
}

func (c *CmdLock) GetDomainName() string {
	return "sp.guild.mod.lock"
}

func (c *CmdLock) GetSubPermissionRules() []shireikan.SubPermission {
	return nil
}

func (c *CmdLock) IsExecutableInDMChannels() bool {
	return false
}

func (c *CmdLock) Exec(ctx shireikan.Context) error {
	db, _ := ctx.GetObject(static.DiDatabase).(database.Database)

	target, err := c.getTargetChan(ctx)
	if err != nil {
		return err
	}

	_, executorID, encodedPerms, err := db.GetLockChan(target.ID)

	if database.IsErrDatabaseNotFound(err) {
		return c.lock(target, ctx, db)
	} else if err == nil {
		return c.unlock(target, ctx, db, executorID, encodedPerms)
	}

	return err
}

func (c *CmdLock) getTargetChan(ctx shireikan.Context) (ch *discordgo.Channel, err error) {
	res := ctx.GetArgs().Get(0).AsString()

	if res == "" {
		ch = ctx.GetChannel()
		return
	}

	ch, err = fetch.FetchChannel(ctx.GetSession(), ctx.GetGuild().ID, res, func(cc *discordgo.Channel) bool {
		return cc.Type == discordgo.ChannelTypeGuildText
	})
	if err != nil {
		return
	}
	if ch == nil {
		err = errors.New("could not fetch any text channel using this resolvable")
	}

	return
}

func (c *CmdLock) lock(target *discordgo.Channel, ctx shireikan.Context, db database.Database) error {
	procMsg := util.SendEmbed(ctx.GetSession(), target.ID, ":clock4: Locking channel...", "", static.ColorEmbedGray)
	if procMsg.Error() != nil {
		return procMsg.Error()
	}

	encodedPerms, err := c.encodePermissionOverrides(target.PermissionOverwrites)
	if err != nil {
		return err
	}

	guildRoles := ctx.GetGuild().Roles
	sort.Slice(guildRoles, func(i, j int) bool {
		return guildRoles[i].Position < guildRoles[j].Position
	})

	memberRoles := ctx.GetMember().Roles

	highest := 0
	rolesMap := make(map[string]*discordgo.Role)
	for _, r := range guildRoles {
		rolesMap[r.ID] = r
		for _, mr := range memberRoles {
			if r.ID != mr {
				continue
			}
			if r.Position > highest {
				highest = r.Position
			}
		}
	}

	// The info message needs to be sent before all permissions are set
	// to prevent occuring errors due to potential missing permissions.
	err = procMsg.Edit(
		fmt.Sprintf("This channel is chat-locked by %s.\nYou may not be able to chat "+
			"into this channel until the channel is unlocked again.", ctx.GetUser().Mention()),
		"", static.ColorEmbedOrange).
		Error()
	if err != nil {
		return err
	}

	st := ctx.GetObject(static.DiState).(*dgrs.State)
	self, err := st.SelfUser()
	if err != nil {
		return err
	}

	hasSetEveryone := false
	for _, po := range target.PermissionOverwrites {
		if po.Type == discordgo.PermissionOverwriteTypeRole {
			if r, ok := rolesMap[po.ID]; ok && r.Position < highest {
				if err = ctx.GetSession().ChannelPermissionSet(
					target.ID, po.ID, discordgo.PermissionOverwriteTypeRole, po.Allow&allowMask, po.Deny|discordgo.PermissionSendMessages); err != nil {
					return err
				}
			}
		}
		if po.Type == discordgo.PermissionOverwriteTypeMember && ctx.GetUser().ID != po.ID && self.ID != po.ID {
			if err = ctx.GetSession().ChannelPermissionSet(
				target.ID, po.ID, discordgo.PermissionOverwriteTypeMember, po.Allow&allowMask, po.Deny|discordgo.PermissionSendMessages); err != nil {
				return err
			}
			if po.ID == target.GuildID {
				hasSetEveryone = true
			}
		}
	}

	if err = ctx.GetSession().ChannelPermissionSet(
		target.ID, self.ID, discordgo.PermissionOverwriteTypeMember, discordgo.PermissionSendMessages&discordgo.PermissionReadMessages, 0); err != nil {
		return err
	}

	if !hasSetEveryone {
		if err = ctx.GetSession().ChannelPermissionSet(
			target.ID, target.GuildID, discordgo.PermissionOverwriteTypeRole, 0, discordgo.PermissionSendMessages); err != nil {
			return err
		}
	}

	if err = db.SetLockChan(target.ID, target.GuildID, ctx.GetUser().ID, encodedPerms); err != nil {
		return err
	}

	return nil
}

func (c *CmdLock) unlock(target *discordgo.Channel, ctx shireikan.Context, db database.Database, executorID, encodedPerms string) error {
	procMsg := util.SendEmbed(ctx.GetSession(), target.ID, ":clock4: Unlocking channel...", "", static.ColorEmbedGray)
	if procMsg.Error() != nil {
		return procMsg.Error()
	}

	permissionOverrides, err := c.decodePermissionOverrrides(encodedPerms)
	if err != nil {
		return err
	}

	failed := 0
	for _, po := range permissionOverrides {
		if err = ctx.GetSession().ChannelPermissionSet(target.ID, po.ID, po.Type, po.Allow, po.Deny); err != nil {
			failed++
		}
	}

	if err = db.DeleteLockChan(target.ID); err != nil {
		return err
	}

	if failed > 0 {
		return procMsg.Edit(
			fmt.Sprintf("This channel is now unlocked. You can now chat here again.\n*(Unlocked by %s)*\n\n"+
				"**Attention:** %d permission actions failed on reset!", ctx.GetUser().Mention(), failed),
			"", static.ColorEmbedOrange).
			Error()
	}

	return procMsg.Edit(
		fmt.Sprintf("This channel is now unlocked. You can now chat here again.\n*(Unlocked by %s)*", ctx.GetUser().Mention()),
		"", static.ColorEmbedGreen).
		Error()
}

func (c *CmdLock) encodePermissionOverrides(po []*discordgo.PermissionOverwrite) (res string, err error) {
	buff := bytes.NewBuffer([]byte{})

	if err = json.NewEncoder(buff).Encode(po); err != nil {
		return
	}

	res = base64.StdEncoding.EncodeToString(buff.Bytes())

	return
}

func (c *CmdLock) decodePermissionOverrrides(data string) (po []*discordgo.PermissionOverwrite, err error) {
	po = make([]*discordgo.PermissionOverwrite, 0)

	dataBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return
	}

	err = json.NewDecoder(bytes.NewBuffer(dataBytes)).Decode(&po)

	return
}
