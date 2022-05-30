package listeners

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/sarulabs/di/v2"
	"github.com/sirupsen/logrus"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/guildlog"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/embedbuilder"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/stringutil"
)

type ListenerMemberAdd struct {
	db database.Database
	gl guildlog.Logger
}

func NewListenerMemberAdd(container di.Container) *ListenerMemberAdd {
	return &ListenerMemberAdd{
		db: container.Get(static.DiDatabase).(database.Database),
		gl: container.Get(static.DiGuildLog).(guildlog.Logger).Section("memberadd"),
	}
}

func (l *ListenerMemberAdd) Handler(s *discordgo.Session, e *discordgo.GuildMemberAdd) {
	autoRoleIDs, err := l.db.GetGuildAutoRole(e.GuildID)
	if err != nil && !database.IsErrDatabaseNotFound(err) {
		logrus.WithError(err).WithField("gid", e.GuildID).Error("Failed getting guild autorole from database")
		l.gl.Errorf(e.GuildID, "Failed getting guild autorole from database: %s", err.Error())
	}
	invalidAutoRoleIDs := make([]string, 0)
	for _, rid := range autoRoleIDs {
		err = s.GuildMemberRoleAdd(e.GuildID, e.User.ID, rid)
		if apiErr, ok := err.(*discordgo.RESTError); ok && apiErr.Message.Code == discordgo.ErrCodeUnknownRole {
			invalidAutoRoleIDs = append(invalidAutoRoleIDs, rid)
		} else if err != nil {
			logrus.WithError(err).WithField("gid", e.GuildID).WithField("uid", e.User.ID).Error("Failed setting autorole for member")
			l.gl.Errorf(e.GuildID, "Failed getting autorole for member (%s): %s", e.User.ID, err.Error())
		}
	}
	if len(invalidAutoRoleIDs) > 0 {
		newAutoRoleIDs := make([]string, 0, len(autoRoleIDs)-len(invalidAutoRoleIDs))
		for _, rid := range autoRoleIDs {
			if !stringutil.ContainsAny(rid, invalidAutoRoleIDs) {
				newAutoRoleIDs = append(newAutoRoleIDs, rid)
			}
		}
		err = l.db.SetGuildAutoRole(e.GuildID, newAutoRoleIDs)
		if err != nil {
			logrus.WithError(err).WithField("gid", e.GuildID).WithField("uid", e.User.ID).Error("Failed updating auto role settings")
			l.gl.Errorf(e.GuildID, "Failed updating auto role settings: %s", e.User.ID, err.Error())
		}
	}

	chanID, msg, err := l.db.GetGuildJoinMsg(e.GuildID)
	if err == nil && msg != "" && chanID != "" {
		txt := ""
		if strings.Contains(msg, "[ment]") {
			txt = e.User.Mention()
		}

		msg = strings.Replace(msg, "[user]", e.User.Username, -1)
		msg = strings.Replace(msg, "[ment]", e.User.Mention(), -1)

		s.ChannelMessageSendComplex(chanID, &discordgo.MessageSend{
			Content: txt,
			Embed: embedbuilder.New().
				WithColor(static.ColorEmbedDefault).
				WithDescription(msg).
				Build(),
		})
	}
}
