package www

import (
	"net/http"

	"github.com/iotalking/mqtt-broker/utils"
)

var mux *utils.Mux

func Start(addr string) error {
	mux = utils.NewHttpMux()
	s := http.Server{}
	s.Addr = addr
	s.Handler = mux
	runter()
	err := s.ListenAndServe()
	if err != nil {
		return err
	}
	return nil
}
