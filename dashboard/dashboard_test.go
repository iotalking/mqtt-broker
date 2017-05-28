package dashboard

import (
	"encoding/json"
	"testing"

	log "github.com/Sirupsen/logrus"
)

func init() {
	log.SetLevel(log.DebugLevel)
}
func TestAtmAdd(t *testing.T) {
	var a AtmI64
	a.Add(1)
	a.Add(1)
	if a != 2 {
		t.Fatalf("RecvMsgCnt != 2,RecvMsgCnt=%d", a)
	}
}
func TestAtmI642AtmType(t *testing.T) {
	var ati AtmType = &Overview.ActiveClients

	t.Log(ati.Get())
}
func TestCopy(t *testing.T) {
	Overview.RunTimeString.Set("hello")
	Overview.ActiveClients.Set(int64(100))
	ov := Overview.Copy()
	t.Logf("ActiveClients :%#v", ov.ActiveClients.Get())
	if ov.ActiveClients.Get().(int64) != 100 {
		t.FailNow()
	}
	if ov.RunTimeString.Get().(string) != "hello" {
		t.Fatalf(`RunTimeString != "hello"`)
	}
}

func TestOverviewToJson(t *testing.T) {

	Overview.RunTimeString.Set("test time")
	bsjson, err := json.MarshalIndent(&Overview, "", "\t")
	if err != nil {
		t.Fatal("overview data json.MarshalIndent error:", err)
		t.FailNow()
	}
	var ov OverviewData
	err = json.Unmarshal(bsjson, &ov)
	if err != nil {
		t.Fatal("overview data json.Unmarshal error:", err)
		t.FailNow()
	}
	if ov.RunTimeString.Get().(string) != "test time" {
		t.Fatal(`RunTimeString != "test time"`)
		t.FailNow()
	}
}
