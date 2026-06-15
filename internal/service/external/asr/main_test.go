package asr

import (
	"os"
	"testing"

	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/logger"
)

func TestMain(m *testing.M) {
	_ = logger.Init(&config.LogConfig{
		Level:    "error",
		Filename: "",
	})
	os.Exit(m.Run())
}
