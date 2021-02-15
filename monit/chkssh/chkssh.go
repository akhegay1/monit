package chkssh

import (
	"bytes"
	"io/ioutil"
	"log"
	"monit/db"
	"monit/mutils"
	"net"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

var Logger *log.Logger

func publicKey(path string) ssh.AuthMethod {
	key, err := ioutil.ReadFile("C:\\Users\\alek\\.ssh\\id_rsa")
	if err != nil {
		panic(err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		panic(err)
	}
	return ssh.PublicKeys(signer)
}

func RunSSHCommand(mId int16, host string, port string, cmd string, vWarning float64, vError float64, username string, password string,
	ch chan<- mutils.Vmetric, Logger *log.Logger, wg *sync.WaitGroup) {
	defer wg.Done()

	Logger.Println("RunSSHCommand tick at")
	//Logger.Println("db.Decrypt(mutils.Key, password) ", db.Decrypt(mutils.Key, password))

	var rslt float64
	config := &ssh.ClientConfig{
		User:            username,
		HostKeyCallback: ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error { return nil }),
		//HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.Password(db.Decrypt(mutils.Key, password)),
		},
	}

	client, err := ssh.Dial("tcp", host+":"+port, config)
	if err != nil {
		Logger.Println("Failed to dial: " + err.Error())
		ch <- mutils.Vmetric{Metric: mId, Value: rslt, Warning: vWarning, Error: vError, Execerr: err.Error()}
		return
	}
	session, err := client.NewSession()
	defer client.Close()
	if err != nil {
		Logger.Println("Failed to create session: " + err.Error())
		ch <- mutils.Vmetric{Metric: mId, Value: rslt, Warning: vWarning, Error: vError, Execerr: err.Error()}
		return
	}

	var b bytes.Buffer
	session.Stdout = &b
	if err := session.Run(cmd); err != nil {
		Logger.Println("Failed to run: " + err.Error())
		ch <- mutils.Vmetric{Metric: mId, Value: rslt, Warning: vWarning, Error: vError, Execerr: err.Error()}
		return
	}
	Logger.Println(b.String())
	rslt, err = strconv.ParseFloat(strings.TrimSpace(b.String()), 32)
	if err != nil {
		Logger.Println("Failed to ParseFloat: " + err.Error())
		ch <- mutils.Vmetric{Metric: mId, Value: rslt, Warning: vWarning, Error: vError, Execerr: err.Error()}
		return
	}

	ch <- mutils.Vmetric{Metric: mId, Value: rslt, Warning: vWarning, Error: vError, Execerr: ""}
	Logger.Println("rslt ssh", rslt)
	//session.Close()

	Logger.Println("ssh after close")
	return

}
