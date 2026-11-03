package config

import "pier/internal/engine/i18n"

// Legacy message assertions exercise the supported Chinese locale explicitly.
func init() { _ = i18n.SetLanguage("zh-CN") }
