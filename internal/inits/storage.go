package inits

import (
	"strings"

	"github.com/sarulabs/di/v2"
	"github.com/sirupsen/logrus"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/config"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/storage"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
)

func InitStorage(container di.Container) storage.Storage {
	var st storage.Storage
	var err error

	cfg := container.Get(static.DiConfig).(config.Provider)

	switch strings.ToLower(cfg.Config().Storage.Type) {
	case "minio", "s3", "googlecloud":
		st = new(storage.Minio)
	case "file":
		st = new(storage.File)
	}

	if err = st.Connect(cfg); err != nil {
		logrus.WithError(err).Fatal("Failed connecting to storage device")
	}

	logrus.Info("Connected to storage device")

	return st
}
