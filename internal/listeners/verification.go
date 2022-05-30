package listeners

import (
	"github.com/bwmarrin/discordgo"
	"github.com/sarulabs/di/v2"
	"github.com/sirupsen/logrus"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/guildlog"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/verification"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
)

type ListenerVerifications struct {
	db database.Database
	vs verification.Provider
	gl guildlog.Logger
}

func NewListenerVerifications(container di.Container) *ListenerVerifications {
	return &ListenerVerifications{
		db: container.Get(static.DiDatabase).(database.Database),
		vs: container.Get(static.DiVerification).(verification.Provider),
		gl: container.Get(static.DiGuildLog).(guildlog.Logger).Section("verification"),
	}
}

func (l *ListenerVerifications) HandlerMemberAdd(s *discordgo.Session, e *discordgo.GuildMemberAdd) {
	if !l.enabled(e.GuildID) {
		return
	}

	err := l.vs.EnqueueVerification(e.GuildID, e.User.ID)
	if err != nil {
		logrus.WithError(err).WithField("gid", e.GuildID).Error("Failed enqueueing user to verification queue")
		l.gl.Errorf(e.GuildID, "Failed enqueueing user to verification queue: %s", err.Error())
	}
}

func (l *ListenerVerifications) HandlerMemberRemove(s *discordgo.Session, e *discordgo.GuildMemberRemove) {

}

func (l *ListenerVerifications) enabled(guildID string) (ok bool) {
	ok, err := l.db.GetGuildVerificationRequired(guildID)
	if err != nil && !database.IsErrDatabaseNotFound(err) {
		logrus.WithError(err).WithField("gid", guildID).Error("Failed getting guild verification state from database")
		l.gl.Errorf(guildID, "Failed getting guild verification state from database: %s", err.Error())
	}
	return
}
