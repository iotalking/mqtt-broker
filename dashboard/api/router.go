package api

import (
	"net/http"

	"github.com/iotalking/mqtt-broker/config"
)

var mux = http.NewServeMux()

func router() {
	mux.HandleFunc(config.DashboordApiUrl+"/overview", GetOverviewData)
	mux.HandleFunc(config.DashboordApiUrl+"/log", SetLogLevel)
	mux.HandleFunc(config.DashboordApiUrl+"/activeSessions", GetSessions)

}
