package controllers

import (
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gofiber/fiber/v2"
	"github.com/sarulabs/di/v2"
	_ "github.com/MK-Mods-OFC/Los-Templarios/internal/models"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/config"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/webserver/auth"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/webserver/v1/models"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/embedded"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/discordutil"
	"github.com/zekrotja/dgrs"
)

type EtcController struct {
	session *discordgo.Session
	cfg     config.Provider
	authMw  auth.Middleware
	st      *dgrs.State
	db      database.Database
}

func (c *EtcController) Setup(container di.Container, router fiber.Router) {
	c.session = container.Get(static.DiDiscordSession).(*discordgo.Session)
	c.cfg = container.Get(static.DiConfig).(config.Provider)
	c.authMw = container.Get(static.DiAuthMiddleware).(auth.Middleware)
	c.st = container.Get(static.DiState).(*dgrs.State)
	c.db = container.Get(static.DiDatabase).(database.Database)

	router.Get("/me", c.authMw.Handle, c.getMe)
	router.Get("/sysinfo", c.getSysinfo)
	router.Get("/privacyinfo", c.getPrivacyinfo)
}

// @Summary Me
// @Description Returns the user object of the currently authenticated user.
// @Tags Etc
// @Accept json
// @Produce json
// @Success 200 {object} models.User
// @Router /me [get]
func (c *EtcController) getMe(ctx *fiber.Ctx) error {
	uid := ctx.Locals("uid").(string)

	user, err := c.st.User(uid)
	if err != nil {
		return err
	}

	caapchaVerified, err := c.db.GetUserVerified(uid)
	if err != nil && !database.IsErrDatabaseNotFound(err) {
		return err
	}

	created, _ := discordutil.GetDiscordSnowflakeCreationTime(user.ID)

	res := &models.User{
		User:            user,
		AvatarURL:       user.AvatarURL(""),
		CreatedAt:       created,
		BotOwner:        uid == c.cfg.Config().Discord.OwnerID,
		CaptchaVerified: caapchaVerified,
	}

	return ctx.JSON(res)
}

// @Summary System Information
// @Description Returns general global system information.
// @Tags Etc
// @Accept json
// @Produce json
// @Success 200 {object} models.SystemInfo
// @Router /sysinfo [get]
func (c *EtcController) getSysinfo(ctx *fiber.Ctx) error {
	buildTS, _ := strconv.Atoi(embedded.AppDate)
	buildDate := time.Unix(int64(buildTS), 0)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	uptime := int64(time.Since(util.StatsStartupTime).Seconds())

	self, err := c.st.SelfUser()
	if err != nil {
		return err
	}

	guilds, err := c.st.Guilds()
	if err != nil {
		return err
	}

	res := &models.SystemInfo{
		Version:    embedded.AppVersion,
		CommitHash: embedded.AppCommit,
		BuildDate:  buildDate,
		GoVersion:  runtime.Version(),

		Uptime:    uptime,
		UptimeStr: fmt.Sprintf("%d", uptime),

		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		CPUs:        runtime.NumCPU(),
		GoRoutines:  runtime.NumGoroutine(),
		StackUse:    memStats.StackInuse,
		StackUseStr: fmt.Sprintf("%d", memStats.StackInuse),
		HeapUse:     memStats.HeapInuse,
		HeapUseStr:  fmt.Sprintf("%d", memStats.HeapInuse),

		BotUserID: self.ID,
		BotInvite: util.GetInviteLink(self.ID),

		Guilds: len(guilds),
	}

	return ctx.JSON(res)
}

// @Summary Privacy Information
// @Description Returns general global privacy information.
// @Tags Etc
// @Accept json
// @Produce json
// @Success 200 {object} models.Privacy
// @Router /privacyinfo [get]
func (c *EtcController) getPrivacyinfo(ctx *fiber.Ctx) error {
	return ctx.JSON(c.cfg.Config().Privacy)
}
