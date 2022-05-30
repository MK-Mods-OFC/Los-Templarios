package messagecommands

import (
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/permissions"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/slashcommands"
	"github.com/zekrotja/ken"
)

type Quote struct {
	slashcommands.Quote
}

var (
	_ ken.MessageCommand      = (*Quote)(nil)
	_ permissions.PermCommand = (*Quote)(nil)
)

func (c *Quote) TypeMessage() {}

func (c *Quote) Name() string {
	return "quotemessage"
}
