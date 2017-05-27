package utils

import (
	"fmt"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
)

type Mux struct {
	*http.ServeMux
}

func NewHttpMux() *Mux {
	return &Mux{
		ServeMux: http.NewServeMux(),
	}
}

func (this *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	this.ServeMux.ServeHTTP(w, r)
}

func DirHandler(pattern, dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var _url = r.URL.Path

		var file = strings.Replace(_url, pattern, dir, 1)
		if file == _url {
			log.Errorf("not math dir")
			//不是相对于dir的url,在程序当前目录找
			file = fmt.Sprintf(".%s", _url)
		}

		log.Infof("HandleDir:r.RequestURI:%s,%#v,file:%s", r.RequestURI, r.URL, file)
		http.ServeFile(w, r, file)
	}
}
func (this *Mux) HandleDir(pattern, dir string) {
	this.ServeMux.HandleFunc(pattern, DirHandler(pattern, dir))
}
