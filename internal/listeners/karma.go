package listeners

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sarulabs/di/v2"
	"github.com/sirupsen/logrus"
	"github.com/zekroTJA/ratelimit"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/guildlog"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/karma"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/zekroTJA/timedmap"
	"github.com/zekrotja/dgrs"
)

const (
	// // reactions used to add or remove karma
	// reactionsAddKarma    = "👍👌⭐✔"
	// reactionsRemoveKarma = "👎❌"

	// duration until a user can differ karma
	// with the same message
	lifetimePerMessage = 24 * time.Hour

	// rateLimiterTokens   = 5                                      // RL bucket size
	// rateLimiterRestore  = time.Hour / rateLimiterTokens          // RL restore duration
	// lifetimeRateLimiter = rateLimiterRestore * rateLimiterTokens // lifetime of a RL in cache
)

const (
	typNull   = 0  // no change
	typAdd    = 1  // add 1 karma
	typRemove = -1 // remove 1 karma
)

type ListenerKarma struct {
	db    database.Database
	gl    guildlog.Logger
	karma *karma.Service
	st    *dgrs.State

	cache       *timedmap.TimedMap
	msgsApplied timedmap.Section
	limiters    timedmap.Section
}

func NewListenerKarma(container di.Container) *ListenerKarma {
	cache := timedmap.New(5 * time.Minute)
	return &ListenerKarma{
		db:    container.Get(static.DiDatabase).(database.Database),
		gl:    container.Get(static.DiGuildLog).(guildlog.Logger).Section("karma"),
		karma: container.Get(static.DiKarma).(*karma.Service),
		st:    container.Get(static.DiState).(*dgrs.State),

		cache: cache,

		// save the pointers to the sections on instance
		// creation to bypass allocations later
		msgsApplied: cache.Section(0),
		limiters:    cache.Section(1),
	}
}

func (l *ListenerKarma) Handler(s *discordgo.Session, e *discordgo.MessageReactionAdd) {
	self, err := l.st.SelfUser()
	if err != nil {
		return
	}

	// Return when reaction was added by the bot itself
	if e.UserID == self.ID {
		return
	}

	// Get karma enabled state for this guild
	if enabled, err := l.karma.GetState(e.GuildID); err != nil {
		logrus.WithError(err).WithField("gid", e.GuildID).Error("Failed getting karma state")
		l.gl.Errorf(e.GuildID, "Failed getting karma state: %s", err.Error())
		return
	} else if !enabled {
		return
	}

	// Get karma emotes
	reactionsAddKarma, reactionsRemoveKarma, err := l.db.GetKarmaEmotes(e.GuildID)
	if err != nil && !database.IsErrDatabaseNotFound(err) {
		logrus.WithError(err).WithField("gid", e.GuildID).Error("Failed getting karma emotes")
		l.gl.Errorf(e.GuildID, "Failed getting karma emotes: %s", err.Error())
		return
	}
	if reactionsAddKarma == "" || reactionsRemoveKarma == "" {
		return
	}

	// Get the type of karma change by the emote used
	var typ int
	if strings.Contains(reactionsAddKarma, e.MessageReaction.Emoji.Name) {
		typ = typAdd
	} else if strings.Contains(reactionsRemoveKarma, e.MessageReaction.Emoji.Name) {
		typ = typRemove
	}

	// When none of the specified emotes was used, return
	if typ == typNull {
		return
	}

	// Check if the executing user is karma blocklisted
	isBlacklisted, err := l.karma.IsBlockListed(e.GuildID, e.UserID)
	if err != nil {
		logrus.WithError(err).WithField("gid", e.GuildID).Error("Failed checking blocklist")
		l.gl.Errorf(e.GuildID, "Failed getting blocklist: %s", err.Error())
		return
	}
	if isBlacklisted {
		return
	}

	// Get the hydrated user object who created the reaction
	user, err := s.User(e.UserID)
	if err != nil {
		logrus.WithError(err).WithField("gid", e.GuildID).WithField("uid", e.UserID).Error("Failed getting user")
		l.gl.Errorf(e.GuildID, "Failed getting user (%s): %s", e.UserID, err.Error())
		return
	}

	// If the user created the reaction is a bot, return
	if user.Bot {
		return
	}

	// Chceck if the message is appliable
	if l.isMessageAlreadyApplied(e.UserID, e.MessageID) {
		return
	}

	// Take a karma token from the users rate limiter
	if !l.rateLimiterTake(e.UserID, e.GuildID) {
		ch, err := s.UserChannelCreate(e.UserID)
		if err == nil {
			util.SendEmbedError(s, ch.ID,
				"You are currently ran out of karma tokens. Please try again later.")
		}
		return
	}

	// Get the hydradet message object where the reaction
	// was added to
	msg, err := l.st.Message(e.ChannelID, e.MessageID)
	if err != nil {
		if msg, err = s.ChannelMessage(e.ChannelID, e.MessageID); err != nil {
			logrus.WithError(err).WithField("gid", e.GuildID).WithField("msg", e.MessageID).Error("Failed getting message")
			l.gl.Errorf(e.GuildID, "Failed getting message (%s): %s", e.MessageID, err.Error())
			return
		}
	}

	// Check if the author of the message is a bot or the
	// message was created by the user created the react.
	// If this is true, return
	if msg.Author.Bot || msg.Author.ID == e.UserID {
		return
	}

	err = l.karma.Update(e.GuildID, msg.Author.ID, e.UserID, typ)
	if err != nil {
		logrus.WithError(err).WithField("gid", e.GuildID).WithField("uid", e.UserID).Error("Failed altering karma value")
		l.gl.Errorf(e.GuildID, "Failed altering karma value (%s): %s", e.UserID, err.Error())
		return
	}

	// Mark the message as applied by the user
	l.applyMessage(e.UserID, e.MessageID)
}

// isMessageAlreadyApplied returns true, if the user has already
// changed karma by reaction to the specified message in the
// time span of lifetimePerMessage.
func (l *ListenerKarma) isMessageAlreadyApplied(userID, msgID string) bool {
	key := fmt.Sprintf("%s:%s", userID, msgID)
	return l.msgsApplied.Contains(key)
}

// applyMessage registers this message as karma change from
// the specified user for the time span of lifetimePerMessage.
func (l *ListenerKarma) applyMessage(userID, msgID string) {
	key := fmt.Sprintf("%s:%s", userID, msgID)
	l.msgsApplied.Set(key, true, lifetimePerMessage)
}

// rateLimiterTake tries to take a ticket from the users
// karma change rate limiter. If all tickets are exceed,
// false will be returned; otherwise the result is true.
func (l *ListenerKarma) rateLimiterTake(userID, guildID string) bool {
	key := fmt.Sprintf("%s:%s", userID, guildID)

	limiter, ok := l.limiters.GetValue(key).(*ratelimit.Limiter)

	rateLimiterTokens, err := l.db.GetKarmaTokens(guildID)
	if err != nil && !database.IsErrDatabaseNotFound(err) {
		logrus.WithError(err).WithField("gid", guildID).Error("Failed getting karma tokens")
		l.gl.Errorf(guildID, "Failed getting karma tokens: %s", err.Error())
		return false
	}
	if rateLimiterTokens < 1 {
		return false
	}

	rateLimiterRestore := time.Hour / time.Duration(rateLimiterTokens)
	lifetimeRateLimiter := rateLimiterRestore * time.Duration(rateLimiterTokens)

	if !ok || limiter == nil {
		limiter = ratelimit.NewLimiter(rateLimiterRestore, rateLimiterTokens)
		l.limiters.Set(key, limiter, lifetimeRateLimiter)
	}

	l.limiters.SetExpires(key, lifetimeRateLimiter)

	return limiter.Allow()
}
