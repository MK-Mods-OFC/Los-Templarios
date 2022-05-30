package listeners

import (
	"encoding/base64"
	"fmt"
	"image/color"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sarulabs/di/v2"
	"github.com/sirupsen/logrus"
	"github.com/zekroTJA/colorname"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/config"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/guildlog"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/permissions"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/colors"
	"github.com/zekroTJA/timedmap"
	"github.com/zekrotja/dgrs"
)

const (
	colorMatchesCap = 5
)

var (
	rxColorHex = regexp.MustCompile(`^#?[\dA-Fa-f]{6,8}$`)
)

type ColorListener struct {
	db         database.Database
	gl         guildlog.Logger
	pmw        *permissions.Permissions
	st         *dgrs.State
	publicAddr string

	emojiCache *timedmap.TimedMap
}

func NewColorListener(container di.Container) *ColorListener {
	cfg := container.Get(static.DiConfig).(config.Provider)
	return &ColorListener{
		db:         container.Get(static.DiDatabase).(database.Database),
		gl:         container.Get(static.DiGuildLog).(guildlog.Logger).Section("colorlistener"),
		pmw:        container.Get(static.DiPermissions).(*permissions.Permissions),
		st:         container.Get(static.DiState).(*dgrs.State),
		publicAddr: cfg.Config().WebServer.PublicAddr,
		emojiCache: timedmap.New(1 * time.Minute),
	}
}

func (l *ColorListener) HandlerMessageCreate(s *discordgo.Session, e *discordgo.MessageCreate) {
	l.process(s, e.Message, false)
}

func (l *ColorListener) HandlerMessageEdit(s *discordgo.Session, e *discordgo.MessageUpdate) {
	l.process(s, e.Message, true)
}

func (l *ColorListener) HandlerMessageReaction(s *discordgo.Session, e *discordgo.MessageReactionAdd) {
	self, err := l.st.SelfUser()
	if err != nil {
		return
	}

	if e.MessageReaction.UserID == self.ID {
		return
	}

	cacheKey := e.MessageID + e.Emoji.ID
	if !l.emojiCache.Contains(cacheKey) {
		return
	}

	clr, ok := l.emojiCache.GetValue(cacheKey).(*color.RGBA)
	if !ok {
		return
	}

	allowed, _, _ := l.pmw.CheckPermissions(s, e.GuildID, e.UserID, "sp.chat.colorreactions")
	if !allowed {
		s.MessageReactionRemove(e.ChannelID, e.MessageID, e.Emoji.APIName(), e.UserID)
		return
	}

	user, err := l.st.User(e.UserID)
	if err != nil {
		return
	}

	hexClr := colors.ToHex(clr)
	intClr := colors.ToInt(clr)
	cC, cM, cY, cK := color.RGBToCMYK(clr.R, clr.G, clr.B)
	yY, yCb, yCr := color.RGBToYCbCr(clr.R, clr.G, clr.B)

	colorName := "*could not be fetched*"
	matches := colorname.FindRGBA(clr)
	if len(matches) > 0 {
		precision := (1 - matches[0].AvgDiff/255) * 100
		colorName = fmt.Sprintf("**%s** *(%0.1f%%)*", matches[0].Name, precision)
	}

	desc := fmt.Sprintf(
		"%s\n\n```\n"+
			"Hex:    #%s\n"+
			"Int:    %d\n"+
			"RGBA:   %03d, %03d, %03d, %03d\n"+
			"CMYK:   %03d, %03d, %03d, %03d\n"+
			"YCbCr:  %03d, %03d, %03d\n"+
			"```",
		colorName,
		hexClr,
		intClr,
		clr.R, clr.G, clr.B, clr.A,
		cC, cM, cY, cK,
		yY, yCb, yCr,
	)

	emb := &discordgo.MessageEmbed{
		Color:       intClr,
		Title:       "#" + hexClr,
		Description: desc,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Activated by " + user.String(),
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: fmt.Sprintf("%s/api/util/color/%s?size=64", l.publicAddr, hexClr),
		},
	}

	_, err = s.ChannelMessageSendComplex(e.ChannelID, &discordgo.MessageSend{
		Embed: emb,
		Reference: &discordgo.MessageReference{
			MessageID: e.MessageID,
			ChannelID: e.ChannelID,
			GuildID:   e.GuildID,
		},
	})
	if err != nil {
		logrus.WithError(err).Error("COLORLISTENER :: could not send embed message")
		l.gl.Errorf(e.GuildID, "Failed sending embed message: %s", err.Error())
	}

	l.emojiCache.Remove(cacheKey)
}

func (l *ColorListener) process(s *discordgo.Session, m *discordgo.Message, removeReactions bool) {
	if len(m.Content) < 6 {
		return
	}

	matches := make([]string, 0)

	content := strings.ReplaceAll(m.Content, "\n", " ")

	// Find color hex in message content using
	// predefined regex.
	for _, v := range strings.Split(content, " ") {
		if rxColorHex.MatchString(v) {
			matches = appendIfUnique(matches, v)
		}
	}

	// Get color reaction enabled guild setting
	// and return when disabled
	active, err := l.db.GetGuildColorReaction(m.GuildID)
	if err != nil && !database.IsErrDatabaseNotFound(err) {
		logrus.WithError(err).Error("COLORLISTENER :: could not get setting from database")
		l.gl.Errorf(m.GuildID, "Could not get setting from database: %s", err.Error())
		return
	}
	if !active {
		return
	}

	cMatches := len(matches)

	// Cancel when no matches were found
	if cMatches == 0 {
		return
	}

	// Cap matches count to colorMatchesCap
	if cMatches > colorMatchesCap {
		matches = matches[:colorMatchesCap]
	}

	if removeReactions {
		if err := s.MessageReactionsRemoveAll(m.ChannelID, m.ID); err != nil {
			logrus.WithError(err).Error("COLORLISTENER :: could not remove previous color reactions")
			l.gl.Errorf(m.GuildID, "Could not remove previous color reactions: %s", err.Error())
		}
	}

	// Execute reaction for each match
	for _, hexClr := range matches {
		l.createReaction(s, m, hexClr)
	}
}

func (l *ColorListener) createReaction(s *discordgo.Session, m *discordgo.Message, hexClr string) {
	// Remove trailing '#' from color code,
	// when existent
	if strings.HasPrefix(hexClr, "#") {
		hexClr = hexClr[1:]
	}

	// Parse hex color code to color.RGBA object
	clr, err := colors.FromHex(hexClr)
	if err != nil {
		logrus.WithError(err).Error("COLORLISTENER :: failed parsing color code")
		l.gl.Errorf(m.GuildID, "Failed parsing color code: %s", err.Error())
		return
	}

	// Create a 24x24 px image with the parsed color
	// rendered as PNG into a buffer
	buff, err := colors.CreateImage(clr, 24, 24)
	if err != nil {
		logrus.WithError(err).Error("COLORLISTENER :: failed generating image data")
		l.gl.Errorf(m.GuildID, "Failed generating color image data: %s", err.Error())
		return
	}

	// Encode the raw image data to a base64 string
	b64Data := base64.StdEncoding.EncodeToString(buff.Bytes())

	// Envelope the base64 data into data uri format
	dataUri := fmt.Sprintf("data:image/png;base64,%s", b64Data)

	// Upload guild emote
	emoji, err := s.GuildEmojiCreate(m.GuildID, hexClr, dataUri, nil)
	if err != nil {
		logrus.WithError(err).Error("COLORLISTENER :: failed uploading emoji")
		l.gl.Errorf(m.GuildID, "Failed uploading emoji: %s", err.Error())
		return
	}

	// Delete the uploaded emote after 5 seconds
	// to give discords caching or whatever some
	// time to save the emoji.
	defer time.AfterFunc(5*time.Second, func() {
		if err = s.GuildEmojiDelete(m.GuildID, emoji.ID); err != nil {
			logrus.WithError(err).Error("COLORLISTENER :: failed deleting emoji")
			l.gl.Errorf(m.GuildID, "Failed deleting emoji: %s", err.Error())
		}
	})

	// Add reaction of the uploaded emote to the message
	err = s.MessageReactionAdd(m.ChannelID, m.ID, emoji.APIName())
	if err != nil {
		logrus.WithError(err).Error("COLORLISTENER :: failed creating message reaction")
		l.gl.Errorf(m.GuildID, "Failed creating message reaction: %s", err.Error())
		return
	}

	// Set messageID + emojiID with RGBA color object
	// to emojiCache
	l.emojiCache.Set(m.ID+emoji.ID, clr, 24*time.Hour)
}

// appendIfUnique appends the given elem to the
// passed slice only if the elem is not already
// contained in slice. Otherwise, slice will
// be returned unchanged.
func appendIfUnique(slice []string, elem string) []string {
	for _, m := range slice {
		if m == elem {
			return slice
		}
	}

	return append(slice, elem)
}
