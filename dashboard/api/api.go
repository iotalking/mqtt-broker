package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/iotalking/mqtt-broker/dashboard"
)

func GetOverviewData(w http.ResponseWriter, r *http.Request) {

	tm := time.Now().Sub(startTime)
	dashboard.Overview.RunTimeString.Set(tm.String())
	dashboard.Overview.RunNanoSeconds.Set(tm.Nanoseconds())
	log.Debug("OverviewData.Get")
	select {
	case getChan <- 0:
	default:
	}
	name := r.FormValue("name")

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "\t")
	d := <-outChan
	if len(name) > 0 {
		log.Debug("name = ", name)

		value := reflect.ValueOf(dashboard.Overview).Elem().FieldByName(r.FormValue("name"))
		if value.IsValid() {

			encoder.Encode(fmt.Sprintf("%v", value))

			log.Info("OverviewData.Get name=", value)
			return
		} else {
			log.Debug("no field of ", name)
		}
	} else {
		log.Debug("name is empty")
	}
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
	log.Debug("dashboard.GetActiveSessions")

	list := sessionMgr.GetSessions()
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "\t")
	encoder.Encode(list)
}
