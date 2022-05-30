package listeners

import (
	"regexp"
	"strings"

	"github.com/sarulabs/di/v2"
	"github.com/sirupsen/logrus"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/guildlog"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/permissions"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/httpreq"

	"github.com/bwmarrin/discordgo"
)

var (
	rxInvLink = regexp.MustCompile(`(?i)(?:https?:\/\/)?(?:www\.)?(?:discord\.gg|discord(?:app)?\.com\/invite)\/(.*)`)
	rxGenLink = regexp.MustCompile(`(?i)(https?:\/\/)?(www\.)?([\w-\S]+\.)+\w{1,10}\/?[\S]+`)
)

type ListenerInviteBlock struct {
	db  database.Database
	gl  guildlog.Logger
	pmw *permissions.Permissions
}

func NewListenerInviteBlock(container di.Container) *ListenerInviteBlock {
	return &ListenerInviteBlock{
		db:  container.Get(static.DiDatabase).(database.Database),
		gl:  container.Get(static.DiGuildLog).(guildlog.Logger).Section("inviteblock"),
		pmw: container.Get(static.DiPermissions).(*permissions.Permissions),
	}
}

func (l *ListenerInviteBlock) HandlerMessageSend(s *discordgo.Session, e *discordgo.MessageCreate) {
	l.invokeCheck(s, e.Message)
}

func (l *ListenerInviteBlock) HandlerMessageEdit(s *discordgo.Session, e *discordgo.MessageUpdate) {
	l.invokeCheck(s, e.Message)
}

func (l *ListenerInviteBlock) invokeCheck(s *discordgo.Session, msg *discordgo.Message) {
	cont := msg.Content

	ok, matches := l.checkForInviteLink(cont)
	if ok {
		l.detected(s, msg, matches)
		return
	}

	link := rxGenLink.FindString(cont)
	if link != "" {
		ok, matches, err := l.followLinkDeep(link, 100, 0)
		if err != nil {
			logrus.WithError(err).WithField("link", link).Error("Failed following link")
			return
		}
		if ok {
			l.detected(s, msg, matches)
		}
	}
}

func (l *ListenerInviteBlock) checkForInviteLink(cont string) (bool, [][]string) {
	matches := rxInvLink.FindAllStringSubmatch(cont, -1)
	return matches != nil, matches
}

func (l *ListenerInviteBlock) followLinkDeep(link string, maxDepth, depth int) (ok bool, matches [][]string, err error) {
	if depth >= maxDepth {
		return
	}

	if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
		link = "http://" + link
	}

	resp, err := httpreq.Get(link, nil)
	if err != nil {
		return
	}
	defer resp.Release()

	code := resp.StatusCode()
	if code < 300 && code > 399 {
		return
	}

	location := string(resp.Header.Peek("location"))
	ok, matches = l.checkForInviteLink(location)

	if !ok {
		return l.followLinkDeep(location, maxDepth, depth+1)
	}

	return ok, matches, nil
}

func (l *ListenerInviteBlock) detected(s *discordgo.Session, e *discordgo.Message, matches [][]string) error {
	enabled, err := l.db.GetGuildInviteBlock(e.GuildID)
	if database.IsErrDatabaseNotFound(err) {
		return nil
	}
	if err != nil || enabled == "" {
		return err
	}

	ok, override, err := l.pmw.CheckPermissions(s, e.GuildID, e.Author.ID, "!sp.guild.mod.inviteblock.send")
	if err != nil || ok || override {
		return err
	}

	if invites, err := s.GuildInvites(e.GuildID); err == nil {
		inviteCode := matches[0][1]
		for _, inv := range invites {
			if inv.Code == inviteCode {
				return nil
			}
		}
	} else {
		logrus.WithError(err).WithField("gid", e.GuildID).Error("INVITEBLOCK :: failed getting guild invites")
		l.gl.Errorf(e.GuildID, "Failed getting guild invites: %s", err.Error())
	}

	if ch, err := s.UserChannelCreate(e.Author.ID); err == nil {
		util.SendEmbedError(s, ch.ID, "Your message contained an invite link to another guild so it has been deleted.")
	}

	return s.ChannelMessageDelete(e.ChannelID, e.ID)
}
