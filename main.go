package main

import (
	"flag"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	log "github.com/Sirupsen/logrus"
	"github.com/djimenez/iconv-go"
	zmq "github.com/pebbe/zmq4"
	_ "golang.org/x/net/html"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	receiverPort = ":5557"
	senderPort   = ":5558"
	maxVisitDeep = 5
)

var (
	urls_file      = flag.String("urls_file", "urls.txt", "seed URL file")
	hander_num     = flag.Int("handler_num", 4, "handler number")
	sconsole_addr  = flag.String("sconsole address", "localhost", "sconsole ip address")
	debug_level    = flag.String("debug_level", "info", "debug level: debug, info, warning, error")
	max_visit_deep = flag.Int("max_visit_deep", maxVisitDeep, "max visit deep")
)

var (
	inQ  chan *Command  = make(chan *Command, 1)
	outQ chan *Command  = make(chan *Command, 1)
	logQ chan *VisitLog = make(chan *VisitLog, 1)
)

type zmqTool struct {
	receiver *zmq.Socket
	sender   *zmq.Socket
}

func newZmqTool() *zmqTool {
	zt := new(zmqTool)
	//var err error
	zt.sender, _ = zmq.NewSocket(zmq.PUSH)
	//defer sender.Close()
	zt.sender.Connect("tcp://" + *sconsole_addr + receiverPort)

	zt.receiver, _ = zmq.NewSocket(zmq.PULL)
	//defer receiver.Close()
	zt.receiver.Connect("tcp://" + *sconsole_addr + senderPort)
	return zt
}
func (zt *zmqTool) close() {
	zt.receiver.Close()
	zt.sender.Close()
}

func main() {
	flag.Parse()
	// Log as JSON instead of the default ASCII formatter.
	//log.SetFormatter(&log.JSONFormatter{})
	log.SetFormatter(&log.TextFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)
	file, err := os.OpenFile("scraper.log", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err == nil {
		//	log.SetOutput(file)
		log.SetOutput(io.MultiWriter(file, os.Stdout))

	} else {
		log.Fatal("Failed to log to file, using default stderr")
	}

	defer file.Close()
	// Only log the warning severity or above.
	fmt.Printf(*debug_level)
	var l log.Level
	switch *debug_level {
	case "debug":
		l = log.DebugLevel
	case "info":
		l = log.InfoLevel
	case "warn":
		l = log.WarnLevel
	case "error":
		l = log.ErrorLevel
	}

	log.SetLevel(l)

	zt := newZmqTool()
	defer zt.close()

	v := runtime.NumCPU()
	log.Debug("NumCPU=", v)
	if *hander_num < v {
		*hander_num = v
	}
	var wg sync.WaitGroup

	for i := 0; i < *hander_num; i++ {
		wg.Add(1)
		go handler(zt, &wg)
	}

	urls, err := ioutil.ReadFile(*urls_file)
	if err != nil {
		log.Error("read urls.txt error, ", err)
		return
	}

	strs := strings.Split(string(urls), "\n")

	cms := newCmdStore(dbName)
	go parseQueue(cms)

	for _, str := range strs {
		if len(str) > 0 {
			u, err := url.Parse(str)
			if err == nil {

				cmd := &Command{Source: "",
					Host:     rootHostname(u.Hostname()),
					Url:      u.String(),
					Accessed: unknown,
					Deep:     0}

				log.Debug("cmd=", cmd)
				inQ <- cmd
			}
		}
	}

	wg.Wait()

	log.Debug("end of main")
}
func parseQueue(cms *cmdStore) {

	var outCmd *Command
	var err error
	for {
		if outCmd == nil {
			log.Debug("outCmd == nil")
			outCmd, err = cms.nextCommand()
			if err != nil || outCmd == nil {
				//should be not found err ontinue
				log.Warnln("should not be found here")

				incmd := <-inQ
				//cms.updateCommand(incmd.Url)
				log.Debug("incmd=", incmd)
				cms.addCommand(incmd)
				outCmd = nil
				continue
			}
			log.Debugln("outCmd=", outCmd)
		}
		select {
		case incmd := <-inQ:
			//log.Debug("incmd=", incmd)
			//cms.updateCommand(incmd.Url)
			cms.addCommand(incmd)
		case outQ <- outCmd:
			log.Debug("outQ<-outCmd,outCmd=", outCmd)
			cms.updateCommand(outCmd.Url)
			outCmd = nil
		case vl := <-logQ:
			log.Debug("vl:=<-log,vl=", vl)
			cms.visitLog(vl)
		}

	}
}

func rootHostname(hostname string) string {

	v := strings.Split(hostname, ".")

	l := len(v)

	if len(v) <= 2 {
		return ""
	}
	return v[l-2] + "." + v[l-1]
}
func handler(zt *zmqTool, wg *sync.WaitGroup) {
	for {
		log.Debug("before s:=<-q")
		cmd := <-outQ
		if cmd == nil {
			log.Debug(" end of cmds ")
			break
		}
		log.Debug("after s:=<-q, s=", cmd.Url)

		u, err := url.Parse(cmd.Url)
		if err != nil {
			log.Errorln("url.Parse err, ", err)
		}
		//zt.sender.Send("will visit "+u.String(), 0)
		client := http.Client{
			Timeout: time.Duration(15 * time.Second),
		}

		//client := http.Client{}

		//log..Info("processing, url=",u.String())
		log.WithFields(log.Fields{
			"url": u.String(),
		}).Info("processing")

		resp, err := client.Get(u.String())
		//defer resp.Body.Close()

		if err != nil {
			log.Errorf("error message: %s", err)
			continue
		}
		v := resp.Header.Get("Content-Type")
		log.WithFields(log.Fields{"charset": v}).Debug("charset")

		cs := strings.Split(v, "=")
		vd := "GBK"

		if len(cs) == 2 {
			vd = cs[1]
		} else {
			log.Errorf("wrong charset,%s,  can not process, continue",v)
			continue
		}
		// Convert the designated charset HTML to utf-8 encoded HTML.
		// `charset` being one of the charsets known by the iconv package.
		utfBody, err := iconv.NewReader(resp.Body, vd, "utf-8")
		if err != nil {
			// handler error
			log.Error("iconv.NewReader return err=", err)
			continue
		}

		//doc, err := goquery.NewDocumentFromResponse(resp)
		doc, err := goquery.NewDocumentFromReader(utfBody)
		if err != nil {
			//log.Debug("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
			log.Error("goquery.NewDocumentFromResponse err=", err)
			continue
		}

		description, _ := doc.Find("head meta[name='description']").Attr("content")
		//log.Debugf("description=%s", description)

		title := doc.Find("html title").Text()

		log.WithFields(log.Fields{"title": title, "description": description}).Info("content received")
		vl := &VisitLog{u.String(), title, description}

		logQ <- vl
		if cmd.Deep >= *max_visit_deep {
			continue
		}
		doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
			val, _ := s.Attr("href")
			//log.Debug("doc.find: val=%s\n", val)
			u, err := u.Parse(val)
			if err != nil {
				log.Errorf("parse failed, href=%s, err=%v",val,err)
				return
			}
			//log.Debug("q len is %di\n", len(q))
			ss := u.String()

			if i := strings.Index(ss, "#"); i != -1 {
				ss = ss[0:i]
			}

			rh := rootHostname(u.Hostname())

			deep := 0
			if rh == cmd.Host {
				deep = cmd.Deep + 1
			}
			inQ <- &Command{cmd.Url,
				rh,
				ss,
				unknown,
				deep}
			//zt.sender.Send(cmd.Url,0)
		})

		//zt.sender.Send(string(content), 0)
		//log.Debug("resp:%s\n",content)
		resp.Body.Close()
		time.Sleep(time.Millisecond)
	}

	wg.Done()
	log.Debug("after wg.Done")

}
