package config

import (
	"fmt"
)

var OverviewTopic = "$sys/overview"

func SessionInfoTopic(sessionId string) string {
	return fmt.Sprintf("$sys/session/%s/info", sessionId)
}
