package chkmetrics

import (
	"log"
	"monit/chkdb"
	"monit/chkssh"
	"monit/chkurl"
	"monit/db"
	"monit/mutils"
	"reflect"
	"runtime"
	"sync"
)

var Logger *log.Logger

//////////////////////////METRICS
type Metric struct {
	Id        int16
	Hostname  string
	Port      string
	Tmetric   int
	Action    string
	Descr     string
	Warning   float64
	Error     float64
	Dbsid     string
	Username  string
	Password  string
	Startm    int8
	Intrvlhrs float64
}

//выбирает из переданного массива канадлов, считывает из него значение и возвращает
//select case будет считывать из каналов пока есть, что считывать
//если в канале кончились то на след канал и т.д.
//ф-ция будет вызвана cnt раз - это кол-во метрик
func multpleSelect(chans []chan mutils.Vmetric) (int, mutils.Vmetric, bool) {
	cases := make([]reflect.SelectCase, len(chans))
	for i, ch := range chans {
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		}
	}

	//из ф-ции reflect.Select(cases) получаем индекс канала, считанное значение, и ок
	//считывается одно значаение из выбранного канала и возвращается
	i, v, ok := reflect.Select(cases)
	Logger.Println("select case v=", v)
	//возвращаем значение метрики (заполненную структуру Vmetric)
	return i, v.Interface().(mutils.Vmetric), ok
}

func GetMetrics() string {
	Logger.Println("getMetrics")

	//Check non crucial metric
	Logger.Println(GetMetricsNotCrucial())

	//var cnt int //for every metric run goroutine and channel
	var cnt int = 3 //for every metrictype create channel to try publish subscribe pattern? multiple goroutins will write only to 3 channels
	err := db.Db.QueryRow(`select count(*) from monit_sch.metrics m,monit_sch.hostnames h 
			where m.hostname=h.id and m.Startm=true and m.intrvlnotcrucialhrs=0`).Scan(&cnt)
	if err != nil {
		Logger.Println(err)
	}

	//when metrcis are checked for all metrics we set int, next time set next seq val
	//last seq val when metrics were checked
	var vnextval int64
	errnxt := db.Db.QueryRow("select nextval('monit_sch.vmetric_last_seq')").Scan(&vnextval)
	if err != nil {
		Logger.Println(errnxt)
	}
	//Logger.Println("vnextval", vnextval)

	rows, err := db.Db.Query(`SELECT m.Id, h.Hostname, Port, Tmetric, Action, Warning, Error,	
								Dbsid, Username, Password, Startm from monit_sch.metrics m,monit_sch.hostnames h
								where m.hostname=h.id and m.Startm=true and m.intrvlnotcrucialhrs=0`)

	if err != nil {
		Logger.Println("select error: ", err)
		return "select error: %v"
	}
	defer rows.Close()

	chans := make([]chan mutils.Vmetric, cnt)
	for i := 0; i < cnt; i++ {
		chans[i] = make(chan mutils.Vmetric, cnt) //используем буфееризованный канал,
		//т.к. блокировка на небуфферизованном канале не дает пройти дальше
		//по идее надо размер буфера ставить в соотв с кол-вом метрик каждого типа,
		//но в думаю и так пойдет, т.к. после выполнения ф-ции все ссылки на этот канал будт очищены и сборщик мусора этот кнала
		//тоже очистит
	}

	Logger.Println("cnt", cnt)
	//для кажд горутины создается канал
	//массив каналов для трех типов каналов - DB, SSH и http get
	var i int = 0
	var wg sync.WaitGroup
	wg.Add(cnt)

	for rows.Next() {
		m := new(Metric)
		err = rows.Scan(&m.Id, &m.Hostname, &m.Port, &m.Tmetric, &m.Action, &m.Warning, &m.Error,
			&m.Dbsid, &m.Username, &m.Password, &m.Startm)
		//Logger.Println("m.Startm", m.Startm)
		if m.Tmetric == 1 {
			//Logger.Println("xxx db metric ")
			go chkdb.GetDbMetric(m.Id, m.Hostname, m.Port, m.Action, m.Dbsid, m.Warning, m.Error, m.Username, m.Password, chans[0], Logger, &wg)
		} else if m.Tmetric == 2 {
			go chkssh.RunSSHCommand(m.Id, m.Hostname, m.Port, m.Action, m.Warning, m.Error, m.Username, m.Password, chans[1], Logger, &wg)
		} else if m.Tmetric == 3 {
			//Logger.Println("xxx url metric ")
			go chkurl.GetUrl(m.Id, m.Hostname, m.Warning, m.Error, chans[2], Logger, &wg)
		}

		i++
	}
	Logger.Println("NumGoroutine=", runtime.NumGoroutine())
	wg.Wait()
	Logger.Println("NumGoroutine=", runtime.NumGoroutine())

	for i := 1; i <= cnt; i++ {
		Logger.Println("chan i=", i)
		if ch, v, ok := multpleSelect(chans); ok {
			Logger.Printf("I am chan-%v, value is %v\n", ch, v)
			//time.Sleep(5 * time.Second)
			if v.Warning > 0 && v.Value > v.Warning {
				Logger.Println("Warning")
			} else if v.Error > 0 && v.Value > v.Error {
				Logger.Println("Error")
			}
			InsMetricVal(v.Metric, v.Value, vnextval, v.Execerr)
		}
	}

	//time.Sleep(40 * time.Second)

	return "success"
}

func InsMetricVal(metric int16, metricval float64, vnextval int64, execerr string) {
	Logger.Printf("metric %d", metric)
	Logger.Printf("metricval %f\n", metricval)
	txn, _ := db.Db.Begin()
	_, err := txn.Exec("INSERT INTO monit_sch.vmetrics (metric, value, lastm, execerr)"+
		" VALUES($1, $2, $3, $4) ", metric, metricval, vnextval, execerr)
	if err != nil {
		Logger.Println("insert error: ", err)
		return
	}
	err = txn.Commit()
	Logger.Println("aft insert")

	if err != nil {
		Logger.Println("insert error: ", err)
		return
	}

}

func GetMetricsNotCrucial() string {
	Logger.Println("GetMetricsNotCrucial")
	var cnt int //for every metric run goroutine and channel

	//select contains count of noncritical metrics which was not checked more than interval
	err := db.Db.QueryRow(`with a as (
		SELECT m.Id, h.Hostname, Port, Tmetric, Action, Warning, Error,	
									Dbsid, Username, Password, Startm, intrvlnotcrucialhrs,
									(SELECT COALESCE(max(vtime), date '2001-01-01') from monit_sch.vmetrics v where v.metric=m.id) maxvtime, now()
									FROM monit_sch.metrics m,monit_sch.hostnames h
									WHERE m.hostname=h.id and m.Startm=true and m.intrvlnotcrucialhrs!=0
									) 
		select count(*) from a where a.intrvlnotcrucialhrs<(EXTRACT(HOUR FROM now()-maxvtime))
		`).Scan(&cnt)
	if err != nil {
		Logger.Println(err)
	}

	if cnt == 0 {
		Logger.Println("0 not crucial metrics")
		return "0 not crucial metrics"
	}

	var vnextval int64
	errnxt := db.Db.QueryRow("select nextval('monit_sch.vmetric_last_seq')").Scan(&vnextval)
	if errnxt != nil {
		Logger.Println(errnxt)
	}
	Logger.Println("vnextval", vnextval)
	//select contains list of noncritical metrics which was not checked more than interval
	rows, err := db.Db.Query(`with a as (
										SELECT m.Id, h.Hostname, Port, Tmetric, Action, Warning, Error,	
										Dbsid, Username, Password, Startm, intrvlnotcrucialhrs,
										(SELECT COALESCE(max(vtime), date '2001-01-01') from monit_sch.vmetrics v where v.metric=m.id) maxvtime, now()
										FROM monit_sch.metrics m,monit_sch.hostnames h
										WHERE m.hostname=h.id and m.Startm=true and m.intrvlnotcrucialhrs!=0
										) 
								select Id, Hostname, Port, Tmetric, Action, Warning, Error,	
							Dbsid, Username, Password, Startm,intrvlnotcrucialhrs from a where a.intrvlnotcrucialhrs<(EXTRACT(HOUR FROM now()-maxvtime))
							`)

	if err != nil {
		Logger.Println("select error: ", err)
		return "select error: %v"
	}
	defer rows.Close()
	Logger.Println("before for")

	chans := make([]chan mutils.Vmetric, cnt)
	for i := 0; i < cnt; i++ {
		chans[i] = make(chan mutils.Vmetric, cnt)
	}

	var wg sync.WaitGroup
	wg.Add(cnt)
	var i int = 0
	for rows.Next() {
		m := new(Metric)
		err = rows.Scan(&m.Id, &m.Hostname, &m.Port, &m.Tmetric, &m.Action, &m.Warning, &m.Error,
			&m.Dbsid, &m.Username, &m.Password, &m.Startm, &m.Intrvlhrs)
		Logger.Println("m.Hostname", m.Hostname)

		go chkdb.GetDbMetric(m.Id, m.Hostname, m.Port, m.Action, m.Dbsid, m.Warning, m.Error, m.Username, m.Password, chans[i], Logger, &wg)

		i++
	}
	wg.Wait()

	for i := 1; i <= cnt; i++ {
		Logger.Println("for multpleSelect")
		if ch, v, ok := multpleSelect(chans); ok {
			Logger.Printf("I am chan-%v, value is %v\n", ch, v)
			if v.Warning > 0 && v.Value > v.Warning {
				Logger.Println("Warning")
			} else if v.Error > 0 && v.Value > v.Error {
				Logger.Println("Error")
			}
			InsMetricVal(v.Metric, v.Value, vnextval, v.Execerr)
		}
	}

	return "success not crucial"
}
