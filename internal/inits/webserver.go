package inits

import (
	"github.com/sarulabs/di/v2"
	"github.com/sirupsen/logrus"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/config"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/webserver"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/mimefix"
)

func InitWebServer(container di.Container) (ws *webserver.WebServer) {

	cfg := container.Get(static.DiConfig).(config.Provider)

	if cfg.Config().WebServer.Enabled {
		curr, ok := mimefix.Check()
		if !ok {
			logrus.Infof("Mime check of .js returned invalid mime value '%s', trying to fix this now ...", curr)
			if err := mimefix.Fix(); err != nil {
				logrus.WithError(err).Error("Fixing .js mime value failed (maybe run as admin to fix this)")
				logrus.Warning("Mime value of .js was not fixed. This may lead to erroneous behaviour of the web server")
			} else {
				logrus.Info("Successfully fixed .js mime value")
			}
		}

		ws, err := webserver.New(container)
		if err != nil {
			logrus.WithError(err).Fatal("Failed initializing web server")
		}

		go func() {
			if err = ws.ListenAndServeBlocking(); err != nil {
				logrus.WithError(err).Fatal("Failed starting up web server")
			}
		}()
		logrus.WithFields(logrus.Fields{
			"bindAddr":   cfg.Config().WebServer.Addr,
			"publicAddr": cfg.Config().WebServer.PublicAddr,
		}).Info("Web server running")
	}
	return
}
