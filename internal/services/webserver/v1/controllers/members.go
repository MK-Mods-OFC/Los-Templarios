package controllers

import (
	"github.com/bwmarrin/discordgo"
	"github.com/gofiber/fiber/v2"
	"github.com/sarulabs/di/v2"
	sharedmodels "github.com/MK-Mods-OFC/Los-Templarios/internal/models"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/config"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/permissions"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/webserver/v1/models"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/webserver/wsutil"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/discordutil"
	"github.com/zekroTJA/shireikan"
	"github.com/zekrotja/dgrs"
)

type GuildMembersController struct {
	session    *discordgo.Session
	cfg        config.Provider
	db         database.Database
	pmw        *permissions.Permissions
	cmdHandler shireikan.Handler
	st         *dgrs.State
}

func (c *GuildMembersController) Setup(container di.Container, router fiber.Router) {
	c.session = container.Get(static.DiDiscordSession).(*discordgo.Session)
	c.cfg = container.Get(static.DiConfig).(config.Provider)
	c.db = container.Get(static.DiDatabase).(database.Database)
	c.pmw = container.Get(static.DiPermissions).(*permissions.Permissions)
	c.cmdHandler = container.Get(static.DiLegacyCommandHandler).(shireikan.Handler)
	c.st = container.Get(static.DiState).(*dgrs.State)

	router.Get("/members", c.getMembers)
	router.Get("/:memberid", c.getMember)
	router.Get("/:memberid/permissions", c.getMemberPermissions)
	router.Get("/:memberid/permissions/allowed", c.getMemberPermissionsAllowed)
	router.Get("/:memberid/reports", c.getReports)
	router.Get("/:memberid/reports/count", c.getReportsCount)
	router.Get("/:memberid/unbanrequests", c.pmw.HandleWs(c.session, "sp.guild.mod.unbanrequests"), c.getMemberUnbanrequests)
	router.Get("/:memberid/unbanrequests/count", c.pmw.HandleWs(c.session, "sp.guild.mod.unbanrequests"), c.getMemberUnbanrequestsCount)
}

// @Summary Get Guild Member List
// @Description Returns a list of guild members.
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "The ID of the guild."
// @Param after query string false "Request members after the given member ID."
// @Param limit query int false "The amount of results returned." default(100) minimum(1) maximum(1000)
// @Success 200 {array} models.Member "Wraped in models.ListResponse"
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 404 {object} models.Error
// @Router /guilds/{id}/members [get]
func (c *GuildMembersController) getMembers(ctx *fiber.Ctx) (err error) {
	uid := ctx.Locals("uid").(string)

	guildID := ctx.Params("guildid")

	memb, _ := c.session.GuildMember(guildID, uid)
	if memb == nil {
		return fiber.ErrNotFound
	}

	after := ""
	limit := 0

	after = ctx.Query("after")
	limit, err = wsutil.GetQueryInt(ctx, "limit", 100, 1, 1000)
	if err != nil {
		return err
	}

	members, err := c.st.Members(guildID)
	if err != nil {
		return err
	}

	if after == "" {
		for i := 0; i < len(members); i++ {
			if members[i].User.ID == after {
				members = members[i+1:]
				break
			}
		}
	}

	if limit > 0 && limit < len(members) {
		members = members[:limit]
	}

	fhmembers := make([]*models.Member, len(members))

	for i, m := range members {
		fhmembers[i] = models.MemberFromMember(m)
	}

	return ctx.JSON(&models.ListResponse{N: len(fhmembers), Data: fhmembers})
}

// @Summary Get Guild Member
// @Description Returns a single guild member by ID.
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "The ID of the guild."
// @Param memberid path string true "The ID of the member."
// @Success 200 {object} models.Member
// @Failure 401 {object} models.Error
// @Failure 404 {object} models.Error
// @Router /guilds/{id}/{memberid} [get]
func (c *GuildMembersController) getMember(ctx *fiber.Ctx) (err error) {
	uid := ctx.Locals("uid").(string)

	guildID := ctx.Params("guildid")
	memberID := ctx.Params("memberid")

	var memb *discordgo.Member

	if memb, _ = c.session.GuildMember(guildID, uid); memb == nil {
		return fiber.ErrNotFound
	}

	guild, err := c.st.Guild(guildID, true)
	if err != nil {
		return err
	}

	memb, _ = c.session.GuildMember(guildID, memberID)
	if memb == nil {
		return fiber.ErrNotFound
	}

	memb.GuildID = guildID

	mm := models.MemberFromMember(memb)

	switch {
	case discordutil.IsAdmin(guild, memb):
		mm.Dominance = 1
	case guild.OwnerID == memberID:
		mm.Dominance = 2
	case c.cfg.Config().Discord.OwnerID == memb.User.ID:
		mm.Dominance = 3
	}

	mm.Karma, err = c.db.GetKarma(memberID, guildID)
	if !database.IsErrDatabaseNotFound(err) && err != nil {
		return err
	}

	mm.KarmaTotal, err = c.db.GetKarmaSum(memberID)
	if !database.IsErrDatabaseNotFound(err) && err != nil {
		return err
	}

	mm.ChatMuted = memb.CommunicationDisabledUntil != nil

	return ctx.JSON(mm)
}

// @Summary Get Guild Member Permissions
// @Description Returns the permission array of the given user.
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "The ID of the guild."
// @Param memberid path string true "The ID of the member."
// @Success 200 {object} models.PermissionsResponse
// @Failure 401 {object} models.Error
// @Failure 404 {object} models.Error
// @Router /guilds/{id}/{memberid}/permissions [get]
func (c *GuildMembersController) getMemberPermissions(ctx *fiber.Ctx) (err error) {
	uid := ctx.Locals("uid").(string)

	guildID := ctx.Params("guildid")
	memberID := ctx.Params("memberid")

	if memb, _ := c.session.GuildMember(guildID, uid); memb == nil {
		return fiber.ErrNotFound
	}

	perm, _, err := c.pmw.GetPermissions(c.session, guildID, memberID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return ctx.JSON(&models.PermissionsResponse{
		Permissions: perm,
	})
}

// @Summary Get Guild Member Allowed Permissions
// @Description Returns all detailed permission DNS which the member is alloed to perform.
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "The ID of the guild."
// @Param memberid path string true "The ID of the member."
// @Success 200 {array} string "Wrapped in models.ListResponse"
// @Failure 401 {object} models.Error
// @Failure 404 {object} models.Error
// @Router /guilds/{id}/{memberid}/permissions/allowed [get]
func (c *GuildMembersController) getMemberPermissionsAllowed(ctx *fiber.Ctx) (err error) {
	guildID := ctx.Params("guildid")
	memberID := ctx.Params("memberid")

	perms, _, err := c.pmw.GetPermissions(c.session, guildID, memberID)
	if database.IsErrDatabaseNotFound(err) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}

	cmds := c.cmdHandler.GetCommandInstances()

	allowed := make([]string, len(cmds)+len(static.AdditionalPermissions))
	i := 0
	for _, cmd := range cmds {
		if perms.Check(cmd.GetDomainName()) {
			allowed[i] = cmd.GetDomainName()
			i++
		}
	}

	for _, p := range static.AdditionalPermissions {
		if perms.Check(p) {
			allowed[i] = p
			i++
		}
	}

	return ctx.JSON(&models.ListResponse{N: i, Data: allowed[:i]})
}

// @Summary Get Guild Member Reports
// @Description Returns a list of reports of the given member.
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "The ID of the guild."
// @Param memberid path string true "The ID of the member."
// @Param limit query int false "The amount of results returned." default(100) minimum(1) maxmimum(100)
// @Param offset query int false "The amount of results to be skipped." default(0)
// @Success 200 {array} models.Report "Wrapped in models.ListResponse"
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 404 {object} models.Error
// @Router /guilds/{id}/{memberid}/reports [get]
func (c *GuildMembersController) getReports(ctx *fiber.Ctx) (err error) {
	uid := ctx.Locals("uid").(string)

	guildID := ctx.Params("guildid")
	memberID := ctx.Params("memberid")

	limit, err := wsutil.GetQueryInt(ctx, "limit", 100, 1, 100)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	offset, err := wsutil.GetQueryInt(ctx, "offset", 0, 0, -1)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if memb, _ := c.session.GuildMember(guildID, uid); memb == nil {
		return fiber.ErrNotFound
	}

	reps, err := c.db.GetReportsFiltered(guildID, memberID, -1, offset, limit)
	if err != nil {
		return err
	}

	resReps := make([]*models.Report, 0)
	if reps != nil {
		resReps = make([]*models.Report, len(reps))
		for i, r := range reps {
			resReps[i] = models.ReportFromReport(r, c.cfg.Config().WebServer.PublicAddr)
			user, err := c.st.User(r.VictimID)
			if err == nil {
				resReps[i].Victim = models.FlatUserFromUser(user)
			}
			user, err = c.st.User(r.ExecutorID)
			if err == nil {
				resReps[i].Executor = models.FlatUserFromUser(user)
			}
		}
	}

	return ctx.JSON(&models.ListResponse{N: len(resReps), Data: resReps})
}

// @Summary Get Guild Member Reports Count
// @Description Returns the total count of reports of the given user.
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "The ID of the guild."
// @Param memberid path string true "The ID of the member."
// @Success 200 {object} models.Count
// @Failure 401 {object} models.Error
// @Failure 404 {object} models.Error
// @Router /guilds/{id}/{memberid}/reports/count [get]
func (c *GuildMembersController) getReportsCount(ctx *fiber.Ctx) (err error) {
	uid := ctx.Locals("uid").(string)

	guildID := ctx.Params("guildid")
	memberID := ctx.Params("memberid")

	if memb, _ := c.session.GuildMember(guildID, uid); memb == nil {
		return fiber.ErrNotFound
	}

	count, err := c.db.GetReportsFilteredCount(guildID, memberID, -1)
	if err != nil {
		return err
	}

	return ctx.JSON(&models.Count{Count: count})
}

// @Summary Get Guild Member Unban Requests
// @Description Returns the list of unban requests of the given member
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "The ID of the guild."
// @Param memberid path string true "The ID of the member."
// @Success 200 {array} sharedmodels.UnbanRequest "Wrapped in models.ListResponse"
// @Failure 401 {object} models.Error
// @Failure 404 {object} models.Error
// @Router /guilds/{id}/{memberid}/unbanrequests [get]
func (c *GuildMembersController) getMemberUnbanrequests(ctx *fiber.Ctx) (err error) {
	guildID := ctx.Params("guildid")
	memberID := ctx.Params("memberid")

	requests, err := c.db.GetGuildUserUnbanRequests(guildID, memberID)
	if err != nil && !database.IsErrDatabaseNotFound(err) {
		return err
	}
	if requests == nil {
		requests = make([]*sharedmodels.UnbanRequest, 0)
	}

	for _, r := range requests {
		r.Hydrate()
	}

	return ctx.JSON(&models.ListResponse{N: len(requests), Data: requests})
}

// @Summary Get Guild Member Unban Requests Count
// @Description Returns the total or filtered count of unban requests of the given member.
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "The ID of the guild."
// @Param memberid path string true "The ID of the member."
// @Param state query sharedmodels.UnbanRequestState false "Filter unban requests by state." default(-1)
// @Success 200 {object} models.Count
// @Failure 401 {object} models.Error
// @Failure 404 {object} models.Error
// @Router /guilds/{id}/{memberid}/unbanrequests/count [get]
func (c *GuildMembersController) getMemberUnbanrequestsCount(ctx *fiber.Ctx) (err error) {
	guildID := ctx.Params("guildid")
	memberID := ctx.Params("memberid")

	stateFilter, err := wsutil.GetQueryInt(ctx, "state", -1, 0, 0)
	if err != nil {
		return err
	}

	requests, err := c.db.GetGuildUserUnbanRequests(guildID, memberID)
	if err != nil && !database.IsErrDatabaseNotFound(err) {
		return err
	}
	if requests == nil {
		requests = make([]*sharedmodels.UnbanRequest, 0)
	}

	count := len(requests)
	if stateFilter > -1 {
		count = 0
		for _, r := range requests {
			if int(r.Status) == stateFilter {
				count++
			}
		}
	}

	return ctx.JSON(&models.Count{Count: count})
}
