package listeners

import (
	"github.com/bwmarrin/discordgo"
	"github.com/sarulabs/di/v2"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
)

type ListenerChannelCreate struct {
	db database.Database
}

func NewListenerChannelCreate(container di.Container) *ListenerChannelCreate {
	return &ListenerChannelCreate{
		db: container.Get(static.DiDatabase).(database.Database),
	}
}

func (l *ListenerChannelCreate) Handler(s *discordgo.Session, e *discordgo.ChannelCreate) {

}
