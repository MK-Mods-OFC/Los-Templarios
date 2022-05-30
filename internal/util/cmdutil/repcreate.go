package cmdutil

import (
	"bytes"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/models"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/config"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/report"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/storage"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/imgstore"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/acceptmsg"
	"github.com/zekrotja/ken"
)

func CmdReport(ctx ken.Context, typ models.ReportType) (err error) {
	cfg := ctx.Get(static.DiConfig).(config.Provider)
	repSvc := ctx.Get(static.DiReport).(*report.ReportService)

	victim := ctx.Options().GetByName("user").UserValue(ctx)
	reason := ctx.Options().GetByName("reason").StringValue()

	var attachment, expire string
	if imageurlV, ok := ctx.Options().GetByNameOptional("imageurl"); ok {
		attachment = imageurlV.StringValue()
	}
	if expireV, ok := ctx.Options().GetByNameOptional("expire"); ok {
		expire = expireV.StringValue()
	}

	if attachment != "" {
		img, err := imgstore.DownloadFromURL(attachment)
		if err == nil && img != nil {
			st, _ := ctx.Get(static.DiObjectStorage).(storage.Storage)
			err = st.PutObject(static.StorageBucketImages, img.ID.String(),
				bytes.NewReader(img.Data), int64(img.Size), img.MimeType)
			if err != nil {
				return err
			}
			attachment = img.ID.String()
		}
	}

	rep := &models.Report{
		GuildID:       ctx.GetEvent().GuildID,
		ExecutorID:    ctx.User().ID,
		VictimID:      victim.ID,
		Msg:           reason,
		AttachmentURL: attachment,
		Type:          typ,
	}

	if expire != "" {
		exp, err := time.ParseDuration(expire)
		if err != nil {
			err = ctx.FollowUpError(
				fmt.Sprintf("Invalid duration:\n```\n%s```", err.Error()), "").Error
			return err
		}
		expT := time.Now().Add(exp)
		rep.Timeout = &expT
	}

	emb := rep.AsEmbed(cfg.Config().WebServer.PublicAddr)
	emb.Title = "Report Check"
	emb.Description = "Is everything okay so far?"

	acceptMsg := acceptmsg.AcceptMessage{
		Embed:          emb,
		Session:        ctx.GetSession(),
		UserID:         ctx.User().ID,
		DeleteMsgAfter: true,
		AcceptFunc: func(msg *discordgo.Message) (err error) {
			switch typ {
			case models.TypeKick:
				rep, err = repSvc.PushKick(rep)
			case models.TypeBan:
				rep, err = repSvc.PushBan(rep)
			default:
				rep, err = repSvc.PushReport(rep)
			}

			if err != nil {
				return
			}

			_, err = ctx.GetSession().ChannelMessageSendEmbed(
				ctx.GetEvent().ChannelID, rep.AsEmbed(cfg.Config().WebServer.PublicAddr))
			return
		},
	}

	if _, err = acceptMsg.AsFollowUp(ctx); err != nil {
		return
	}
	return acceptMsg.Error()
}
