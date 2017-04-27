package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"

	"github.com/iotalking/mqtt-broker/config"
)

var mux = http.NewServeMux()

func router() {
	mux.HandleFunc(config.DashboordApiUrl+"/overview", Overview.Get)
	mux.HandleFunc(config.DashboordApiUrl+"/log", SetLogLevel)
	mux.HandleFunc(config.DashboordApiUrl+"/broker", GetSessions)
}

func run() {
	router()
	s := http.Server{}
	s.Addr = fmt.Sprintf(":%d", config.RestfulPort)
	s.Handler = mux
	go func() {
		log.Debug("dashboad http started")
		log.Println(s.ListenAndServe())
	}()
	for {
		select {
		case <-Overview.getChan:
			log.Debug("geting overview data")
			Overview.outChan <- *Overview
		}
	}
}
func (this *OverviewData) Get(w http.ResponseWriter, r *http.Request) {
	log.Debug("OverviewData.Get")
	select {
	case this.getChan <- 0:
	default:
	}

	d := <-this.outChan
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "\t")
	encoder.Encode(d)
}

func SetLogLevel(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			http.Error(w, "level=<panic|fatal|error|warn|warning|info|debug>", http.StatusBadRequest)
		}
	}()
	err = r.ParseForm()
	if err != nil {
		log.Error("dashboard.SetLogLevel error:", err)
		return
	}
	v, err := log.ParseLevel(r.FormValue("level"))
	if err != nil {
		log.Error("dashboard.ParseLevel error:", err)
		return
	}
	log.SetLevel(v)
}

func GetSessions(w http.ResponseWriter, r *http.Request) {
	log.Debug("dashboard.GetSessions")
	list := sessionMgr.GetSessions()
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "\t")
	encoder.Encode(list)
}
