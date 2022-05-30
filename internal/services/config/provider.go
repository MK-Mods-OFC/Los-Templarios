package config

import (
	oldconfig "github.com/MK-Mods-OFC/Los-Templarios/internal/models"
)

type Provider interface {
	Config() *oldconfig.Config
	Parse() error
}
