package usercommands

import (
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/permissions"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/slashcommands"
	"github.com/zekrotja/ken"
)

type User struct {
	slashcommands.User
}

var (
	_ ken.UserCommand         = (*User)(nil)
	_ permissions.PermCommand = (*User)(nil)
)

func (c *User) TypeUser() {}

func (c *User) Name() string {
	return "userinfo"
}
