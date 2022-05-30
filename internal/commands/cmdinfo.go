package commands

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/embedded"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/zekroTJA/shireikan"
	"github.com/zekrotja/dgrs"

	_ "embed"
)

//go:embed embed/cmd_info.md
var infoMsg string

type CmdInfo struct {
}

func (c *CmdInfo) GetInvokes() []string {
	return []string{"info", "information", "description", "credits", "version", "invite"}
}

func (c *CmdInfo) GetDescription() string {
	return "Display some information about this bot."
}

func (c *CmdInfo) GetHelp() string {
	return "`info`"
}

func (c *CmdInfo) GetGroup() string {
	return shireikan.GroupGeneral
}

func (c *CmdInfo) GetDomainName() string {
	return "sp.etc.info"
}

func (c *CmdInfo) GetSubPermissionRules() []shireikan.SubPermission {
	return nil
}

func (c *CmdInfo) IsExecutableInDMChannels() bool {
	return true
}

func (c *CmdInfo) Exec(ctx shireikan.Context) error {
	st := ctx.GetObject(static.DiState).(*dgrs.State)
	self, err := st.SelfUser()
	if err != nil {
		return err
	}

	invLink := util.GetInviteLink(self.ID)

	emb := &discordgo.MessageEmbed{
		Color: static.ColorEmbedDefault,
		Title: "Info",
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: self.AvatarURL(""),
		},
		Description: infoMsg,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Repository",
				Value: "[github.com/MK-Mods-OFC/Los-Templarios](https://github.com/MK-Mods-OFC/Los-Templarios)",
			},
			{
				Name: "Version",
				Value: fmt.Sprintf("This instance is running on version **%s** (commit hash `%s`)",
					embedded.AppVersion, embedded.AppCommit),
			},
			{
				Name:  "Licence",
				Value: "Covered by the [MIT Licence](https://github.com/MK-Mods-OFC/Los-Templarios/blob/master/LICENCE).",
			},
			{
				Name: "Invite",
				Value: fmt.Sprintf("[Invite Link](%s).\n```\n%s\n```",
					invLink, invLink),
			},
			{
				Name:  "Bug Hunters",
				Value: "Much :heart: to all [**bug hunters**](https://github.com/MK-Mods-OFC/Los-Templarios/blob/dev/bughunters.md).",
			},
			{
				Name:  "Development state",
				Value: "You can see current tasks [here](https://github.com/MK-Mods-OFC/Los-Templarios/projects).",
			},
			{
				Name: "3rd Party Dependencies and Credits",
				Value: "[Here](https://github.com/MK-Mods-OFC/Los-Templarios/blob/master/README.md#third-party-dependencies) you can find a list of all dependencies used.\n" +
					"Avatar of [御中元 魔法少女詰め合わせ](https://www.pixiv.net/member_illust.php?mode=medium&illust_id=44692506) from [瑞希](https://www.pixiv.net/member.php?id=137253).",
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("© 2018-%s zekro Development (Ringo Hoffmann)", time.Now().Format("2006")),
		},
	}
	_, err = ctx.GetSession().ChannelMessageSendEmbed(ctx.GetChannel().ID, emb)
	return err
}
