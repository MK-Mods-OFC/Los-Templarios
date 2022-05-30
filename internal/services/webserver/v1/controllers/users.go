package controllers

import (
	"github.com/bwmarrin/discordgo"
	"github.com/gofiber/fiber/v2"
	"github.com/sarulabs/di/v2"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/config"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/webserver/auth"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/webserver/v1/models"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/discordutil"
	"github.com/zekrotja/dgrs"
)

type UsersController struct {
	session *discordgo.Session
	cfg     config.Provider
	authMw  auth.Middleware
	st      *dgrs.State
}

func (c *UsersController) Setup(container di.Container, router fiber.Router) {
	c.session = container.Get(static.DiDiscordSession).(*discordgo.Session)
	c.cfg = container.Get(static.DiConfig).(config.Provider)
	c.authMw = container.Get(static.DiAuthMiddleware).(auth.Middleware)
	c.st = container.Get(static.DiState).(*dgrs.State)

	router.Get(":id", c.getUser)
}

// @Summary User
// @Description Returns the information of a user by ID.
// @Tags Users
// @Accept json
// @Produce json
// @Success 200 {object} models.User
// @Router /users/{id} [get]
func (c *UsersController) getUser(ctx *fiber.Ctx) error {
	uid := ctx.Params("id")

	user, err := c.st.User(uid)
	if err != nil {
		return err
	}

	created, _ := discordutil.GetDiscordSnowflakeCreationTime(user.ID)

	res := &models.User{
		User:      user,
		AvatarURL: user.AvatarURL(""),
		CreatedAt: created,
		BotOwner:  uid == c.cfg.Config().Discord.OwnerID,
	}

	return ctx.JSON(res)
}
