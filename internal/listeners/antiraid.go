package listeners

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sarulabs/di/v2"
	"github.com/sirupsen/logrus"
	"github.com/zekroTJA/ratelimit"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/guildlog"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/verification"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/antiraid"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/discordutil"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/voidbuffer/v2"
	"github.com/zekroTJA/timedmap"
	"github.com/zekrotja/dgrs"
)

const (
	arTriggerCleanupDuration = 1 * time.Hour
)

type guildState struct {
	rl *ratelimit.Limiter
	bf *voidbuffer.VoidBuffer[string]
}

type ListenerAntiraid struct {
	db database.Database
	gl guildlog.Logger
	st *dgrs.State
	vs verification.Provider

	guildStates map[string]*guildState
	triggers    *timedmap.TimedMap
}

func NewListenerAntiraid(container di.Container) *ListenerAntiraid {
	return &ListenerAntiraid{
		db:          container.Get(static.DiDatabase).(database.Database),
		guildStates: make(map[string]*guildState),
		triggers:    timedmap.New(arTriggerCleanupDuration),
		gl:          container.Get(static.DiGuildLog).(guildlog.Logger).Section("antiraid"),
		st:          container.Get(static.DiState).(*dgrs.State),
		vs:          container.Get(static.DiVerification).(verification.Provider),
	}
}

func (l *ListenerAntiraid) addToJoinlog(e *discordgo.GuildMemberAdd) {
	creation, err := discordutil.GetDiscordSnowflakeCreationTime(e.User.ID)
	if err != nil {
		logrus.WithError(err).WithField("gid", e.GuildID).WithField("uid", e.User.ID).Error("Failed getting creation date from user snowflake")
		l.gl.Errorf(e.GuildID, "Failed getting creation date from user snowflake (%s): %s", e.User.ID, err.Error())
	} else if err = l.db.AddToAntiraidJoinList(e.GuildID, e.User.ID, e.User.String(), creation); err != nil {
		logrus.WithError(err).WithField("gid", e.GuildID).WithField("uid", e.User.ID).Error("Failed adding user to joinlist")
		l.gl.Errorf(e.GuildID, "Failed adding user to joinlist (%s): %s", e.User.ID, err.Error())
	}
}

func (l *ListenerAntiraid) HandlerMemberAdd(s *discordgo.Session, e *discordgo.GuildMemberAdd) {
	if v, ok := l.triggers.GetValue(e.GuildID).(time.Time); ok {
		if time.Since(v) < antiraid.TriggerRecordLifetime {
			l.addToJoinlog(e)
		}
		return
	}

	ok, limit, burst := l.getGuildSettings(e.GuildID)
	if !ok {
		if _, ok := l.guildStates[e.GuildID]; ok {
			delete(l.guildStates, e.GuildID)
		}
		return
	}

	limitDur := time.Duration(limit) * time.Second

	state, ok := l.guildStates[e.GuildID]
	if !ok || state == nil {
		state = &guildState{
			rl: ratelimit.NewLimiter(limitDur, burst),
			bf: voidbuffer.New[string](50),
		}
		l.guildStates[e.GuildID] = state
	} else {
		if state.rl.Burst() != burst {
			state.rl.SetBurst(burst)
		}
		if state.rl.Limit() != limitDur {
			state.rl.SetLimit(limitDur)
		}
	}

	if state.bf.Contains(e.User.ID) {
		return
	}

	state.bf.Push(e.User.ID)

	if state.rl.Allow() {
		return
	}

	verificationLvl := discordgo.VerificationLevelVeryHigh
	_, err := s.GuildEdit(e.GuildID, discordgo.GuildParams{
		VerificationLevel: &verificationLvl,
	})

	guild, err := l.st.Guild(e.GuildID, true)
	if err != nil {
		logrus.WithError(err).WithField("gid", e.GuildID).Error("Failed getting guild")
		return
	}

	alertDescrition := fmt.Sprintf(
		"Following guild you are admin on is currently being raided!\n\n"+
			"**%s (`%s`)**\n\n"+
			"Because an atypical burst of members joined the guild, "+
			"the guilds verification level was raised to `verry high` and all admins "+
			"were informed.\n\n"+
			"Also, all joining users from now are saved in a log list for the following "+
			"24 hours. This log is saved for 48 hours toal.", guild.Name, e.GuildID)
	if err != nil {
		alertDescrition = fmt.Sprintf("%s\n\n"+
			"**Attention:** Failed to raise guilds verification level because "+
			"following error occured:\n```\n%s\n```", alertDescrition, err.Error())
	}

	emb := &discordgo.MessageEmbed{
		Title:       "⚠ GUILD RAID ALERT",
		Description: alertDescrition,
		Color:       static.ColorEmbedOrange,
	}

	members, err := l.st.Members(e.GuildID)
	if err != nil {
		logrus.WithError(err).WithField("gid", e.GuildID).Error("Failed getting guild members")
		l.gl.Errorf(e.GuildID, "Failed getting guild members: %s", err.Error())
		return
	}

	l.triggers.Set(e.GuildID, time.Now(), antiraid.TriggerLifetime, func(v interface{}) {
		if err = l.db.FlushAntiraidJoinList(e.GuildID); err != nil && !database.IsErrDatabaseNotFound(err) {
			logrus.WithError(err).WithField("gid", e.GuildID).Error("Failed flushing joinlist")
			l.gl.Errorf(e.GuildID, "Failed flushing joinlist: %s", err.Error())
		}
	})

	for _, m := range members {
		if discordutil.IsAdmin(guild, m) || guild.OwnerID == m.User.ID {
			ch, err := s.UserChannelCreate(m.User.ID)
			if err != nil {
				continue
			}
			s.ChannelMessageSendEmbed(ch.ID, emb)
		}
	}

	if chanID, _ := l.db.GetGuildModLog(e.GuildID); chanID != "" {
		s.ChannelMessageSendEmbed(chanID, &discordgo.MessageEmbed{
			Color: static.ColorEmbedOrange,
			Title: "⚠ GUILD RAID ALERT",
			Description: "Because an atypical burst of members joined the guild, " +
				"the guilds verification level was raised to `verry high` and all admins " +
				"were informed.\n\n" +
				"Also, all joining users from now are saved in a log list for the following " +
				"24 hours. This log is saved for 48 hours toal.",
		})
	}

	ok, err = l.db.GetAntiraidVerification(e.GuildID)
	if err != nil {
		logrus.WithError(err).WithField("gid", e.GuildID).Error("Failed gettings antiraid verification state")
		l.gl.Errorf(e.GuildID, "Failed gettings antiraid verification state: %s", err.Error())
	}
	if ok {
		err = l.vs.SetEnabled(e.GuildID, true)
		if err != nil {
			logrus.WithError(err).WithField("gid", e.GuildID).Error("Failed enabling verification")
			l.gl.Errorf(e.GuildID, "Failed enabling verification: %s", err.Error())
		}
	}

	l.addToJoinlog(e)
}

func (l *ListenerAntiraid) getGuildSettings(gid string) (ok bool, limit, burst int) {
	var err error
	var state bool

	state, err = l.db.GetAntiraidState(gid)
	if err != nil && !database.IsErrDatabaseNotFound(err) {
		logrus.WithError(err).WithField("gid", gid).Error("Failed getting antiraid state")
		l.gl.Errorf(gid, "Failed getting antiraid state: %s", err.Error())
		return
	}
	if !state {
		return
	}

	limit, err = l.db.GetAntiraidRegeneration(gid)
	if err != nil && !database.IsErrDatabaseNotFound(err) {
		logrus.WithError(err).WithField("gid", gid).Error("Failed getting antiraid regeneration")
		l.gl.Errorf(gid, "Failed getting antiraid regeneration: %s", err.Error())
		return
	}
	if limit < 1 {
		return
	}

	burst, err = l.db.GetAntiraidBurst(gid)
	if err != nil && !database.IsErrDatabaseNotFound(err) {
		logrus.WithError(err).WithField("gid", gid).Error("Failed getting antiraid burst")
		l.gl.Errorf(gid, "Failed getting antiraid burst: %s", err.Error())
		return
	}
	if burst < 1 {
		return
	}

	ok = true

	return
}
