package chkurl

import (
	"log"
	"monit/pkg/mutils"
	"net/http"
	"sync"
)

func GetUrl(mId int16, host string, vWarning float64, vError float64, ch chan<- mutils.Vmetric, Logger *log.Logger, wg *sync.WaitGroup) {
	Logger.Println("host", host)
	defer wg.Done()
	var rslt float64
	resp, err := http.Get(host)
	// handle the error if there is one
	if err != nil {
		ch <- mutils.Vmetric{Metric: mId, Value: rslt, Warning: vWarning, Error: vError, Execerr: err.Error()}
		return
	}
	defer resp.Body.Close()

	ch <- mutils.Vmetric{Metric: mId, Value: float64(resp.StatusCode), Warning: vWarning, Error: vError, Execerr: ""}
	Logger.Println("resp.StatusCode", resp.StatusCode)
	return
}
