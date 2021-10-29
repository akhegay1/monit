package main

import (
	"bufio"
	"log"
	"monit/internal/chkdb"
	"monit/internal/chkmetrics"
	"monit/internal/chkssh"
	"monit/internal/db"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/godror/godror"
	_ "github.com/lib/pq"
)

var errorlog *os.File
var logger *log.Logger
var words []string

func init() {
	errorlog, err := os.OpenFile("monit.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		logger.Printf("error opening file: %v", err)
		os.Exit(1)
	}
	//defer errorlog.Close()
	logger = log.New(errorlog, "applog: ", log.Lshortfile|log.LstdFlags)
	logger.Println("main")

	db.Logger = logger
	chkmetrics.Logger = logger
	chkdb.Logger = logger
	chkssh.Logger = logger

	file, _ := exec.LookPath(os.Args[0])
	path, _ := filepath.Abs(file)
	index := strings.LastIndex(path, string(os.PathSeparator))
	logger.Println("path: %s", path)
	logger.Println("index: %s", index)
	pwd, _ := os.Getwd()

	//REED PARAMS
	conf, err := os.Open(pwd + "/params.conf")
	if err != nil {
		logger.Println("failed opening file conf: %s", err)
	}
	defer conf.Close()

	sc := bufio.NewScanner(conf)

	for sc.Scan() {
		str := sc.Text() // GET the line string
		words = strings.Fields(str)
	}

	if err := sc.Err(); err != nil {
		logger.Println("scan file error: %v", err)
	}

	//fmt.Println("words", words)
}

//ssss
//ddd
func main() {
	c := db.Connect()
	logger.Println(c)

	var m string
	interval_sec, _ := strconv.ParseInt(words[1], 0, 8)
	ticker := time.NewTicker(time.Second * time.Duration(interval_sec))

	m = chkmetrics.GetMetrics()

	for t := range ticker.C {
		m = chkmetrics.GetMetrics()
		logger.Println(m)
		logger.Println("chkmetrics Tick at", t)
	}

	logger.Println("--------------------------------------------------------------------------")
}
