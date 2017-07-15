package api

import (
	"net/http"

	"github.com/iotalking/mqtt-broker/config"
	dashboardapi "github.com/iotalking/mqtt-broker/dashboard/api"
)

var mux = http.NewServeMux()

func router() {
	mux.HandleFunc(config.DashboordApiUrl+"/overview", dashboardapi.GetOverviewData)
	mux.HandleFunc(config.DashboordApiUrl+"/log", dashboardapi.SetLogLevel)
	mux.HandleFunc(config.DashboordApiUrl+"/activeSessions", dashboardapi.GetSessions)
	mux.HandleFunc(config.RestfullApiUrl+"/pub", Pub)

}
