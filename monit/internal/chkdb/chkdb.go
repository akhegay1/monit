package chkdb

import (
	"database/sql"
	"log"
	"monit/internal/db"
	"monit/pkg/mutils"
	"sync"
)

var dbc *sql.DB
var Logger *log.Logger

func GetDbMetric(mId int16, host string, port string, action string, dbsid string, vWarning float64, vError float64, user string, password string,
	ch chan<- mutils.Vmetric, Logger *log.Logger, wg *sync.WaitGroup) {
	Logger.Println("GetDbMetric")
	defer wg.Done()

	var rslt float64

	connstr := host + ":" + port + "/" + dbsid
	passwd := user + "/" + db.Decrypt(mutils.Key, password)
	//Logger.Println(connstr)
	//Logger.Println(passwd)

	db, err := sql.Open("godror", passwd+"@"+connstr)
	Logger.Println("aft sql.open")
	//Logger.Printf("I am chan-%v", ch)
	defer db.Close()
	if err != nil {
		Logger.Println(err)
		ch <- mutils.Vmetric{Metric: mId, Value: rslt, Warning: vWarning, Error: vError, Execerr: err.Error()}
		//defer db.Close()
		return
	}

	row := db.QueryRow(action)
	err = row.Scan(&rslt)
	Logger.Println("err", err)

	if err != nil {
		Logger.Println("Error fetching user data\n", err)
		ch <- mutils.Vmetric{Metric: mId, Value: rslt, Warning: vWarning, Error: vError, Execerr: err.Error()}
		//defer db.Close()
		return
	}
	Logger.Println("rslt", rslt)
	ch <- mutils.Vmetric{Metric: mId, Value: rslt, Warning: vWarning, Error: vError, Execerr: ""}

}
