package www

import (
	"mime"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/iotalking/mqtt-broker/utils"
)

func runter() {
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".html", "text/html")
	mime.AddExtensionType(".png", "image/png")

	mux.HandleDir("/www/client/", "./www/client/")
	mux.HandleDir("/www/libs/", "./www/libs/")

	HandleWhiteBoard("/www/whiteboard/", "./www/whiteboard/")
}
func HandleWhiteBoard(pattern, dir string) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {

		var _url = r.URL.Path
		if _url == pattern {
			//创建白板name
			ck := http.Cookie{
				Name:  "name",
				Value: utils.NewId(),
			}
			log.Info("HandleWhiteBoard")
			http.SetCookie(w, &ck)
		}
		utils.DirHandler(pattern, dir)(w, r)
	})

}
