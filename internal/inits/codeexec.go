package inits

import (
	"strings"

	"github.com/sarulabs/di/v2"
	"github.com/sirupsen/logrus"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/codeexec"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/config"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
)

func InitCodeExec(container di.Container) codeexec.Factory {
	cfg := container.Get(static.DiConfig).(config.Provider)

	switch strings.ToLower(cfg.Config().CodeExec.Type) {

	case "ranna":
		exec, err := codeexec.NewRannaFactory(container)
		if err != nil {
			logrus.WithError(err).Fatal("failed setting up ranna factroy")
		}
		return exec

	default:
		return codeexec.NewJdoodleFactory(container)
	}
}
