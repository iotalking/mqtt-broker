package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	log "github.com/Sirupsen/logrus"
)

var localId string

func init() {
	netifs, err := net.Interfaces()
	md5hash := md5.New()
	if err != nil {
		panic(err)
	}
	for _, nif := range netifs {
		mac := nif.HardwareAddr.String()
		log.Debug("%s\n", mac)
		md5hash.Write([]byte(mac))
	}
	localId = hex.EncodeToString(md5hash.Sum(nil))
	log.Info("localId:%s\n", localId)
}
func NewId() string {
	sum := md5.Sum([]byte(fmt.Sprintf("%s%d", localId, time.Now().UnixNano())))
	return hex.EncodeToString(sum[:])

}

func LocalId() string {
	return localId
}
