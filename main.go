package main

import (
	"flag"
	"github.com/PuerkitoBio/goquery"
	log "github.com/Sirupsen/logrus"
	zmq "github.com/pebbe/zmq4"
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
)

var (
	urls_file     = flag.String("urls_file", "urls.txt", "seed URL file")
	hander_num    = flag.Int("handler_num", 4, "handler number")
	sconsole_addr = flag.String("sconsole address", "localhost", "sconsole ip address")
)

var (
	in   chan *Command  = make(chan *Command, 1)
	out  chan *Command  = make(chan *Command, 1)
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
func init() {
	// Log as JSON instead of the default ASCII formatter.
	//log.SetFormatter(&log.JSONFormatter{})
	log.SetFormatter(&log.TextFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	//log.SetOutput(os.Stdout)
	file, err := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY, 0666)
	if err == nil {
		log.SetOutput(file)
	} else {
		log.Info("Failed to log to file, using default stderr")
	}

	// Only log the warning severity or above.
	log.SetLevel(log.DebugLevel)
}

func main() {
	flag.Parse()

	zt := newZmqTool()
	defer zt.close()

	v := runtime.NumCPU()
	log.Debug("NumCPU=", v)
	if *hander_num > v {
		*hander_num = v
	}
	//q := make(chan *url.URL, 1e8)

	//for test
	//*hander_num = 10
	var wg sync.WaitGroup
	for i := 0; i < *hander_num; i++ {
		wg.Add(1)
		go handler(zt, &wg)
	}

	urls, err := ioutil.ReadFile(*urls_file)
	if err != nil {
		log.Debug("read urls.txt error, ", err)
		return
	}

	strs := strings.Split(string(urls), "\n")

	cms := newCmdStore(dbName)
	go parseQueue(cms)

	for _, str := range strs {
		log.Debug("before q<-str, str=", str)
		if len(str) > 0 {
			u, err := url.Parse(str)
			if err == nil {

				cmd := &Command{Source: "", Url: u.String(), Accessed: unknown}

				log.Debug("cmd=", cmd)
				in <- cmd
			}
		}
	}

	/*
		for i := 0; i < *hander_num; i++ {
			q <- nil
		}*/

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
			log.Debug("outCmd=", outCmd)
			if err != nil {
				//should be not found err ontinue
				log.Debug("should not be found here")

				incmd := <-in
				//cms.updateCommand(incmd.Url)
				log.Debug("incmd=", incmd)
				cms.addCommand(incmd)
				outCmd = nil
				continue
			}
		}
		select {
		case incmd := <-in:
			//log.Debug("incmd=", incmd)
			//cms.updateCommand(incmd.Url)
			cms.addCommand(incmd)
		case out <- outCmd:
			log.Debug("out<-outCmd,outCmd=", outCmd)
			cms.updateCommand(outCmd.Url)
			outCmd = nil
		case vl := <-logQ:
			log.Debug("vl:=<-log,vl=", vl)
			cms.visitLog(vl)
		}

	}
}
func handler(zt *zmqTool, wg *sync.WaitGroup) {
	for {
		log.Debug("before s:=<-q")
		cmd := <-out
		if cmd == nil {
			log.Debug(" end of cmds ")
			break
		}
		log.Debug("after s:=<-q, s=", cmd.Url)

		u, err := url.Parse(cmd.Url)
		if err != nil {
			log.Debug("url.Parse err, ", err)
		}
		//zt.sender.Send("will visit "+u.String(), 0)
		client := http.Client{
			Timeout: time.Duration(15 * time.Second),
		}

		//client := http.Client{}

		log.Debug("before Get, url=",u.String())
		resp, err := client.Get(u.String())
		//defer resp.Body.Close()

		if err != nil {
			log.Debug("error message: %s\n", err)
			continue
		}

		doc, err := goquery.NewDocumentFromResponse(resp)
		if err != nil {
			//log.Debug("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
			log.Debug("goquery.NewDocumentFromResponse err=", err)
			continue
		}

		title := doc.Find("html title").Text()
		log.Debug("title=%s\n", title)

		description, _ := doc.Find("head meta[name='description']").Attr("content")
		log.Debug("description=%s\n", description)
		/*for host,deep:=range f.deep{
			log.Debug("crew deep =%d, host=%s",deep,host)

		}

		if(f.deep[cmd.u.Host]>=f.deepLimit){
			log.Debug("crew deep reached, host=%s",cmd.u.Host)
			return nil
		}*/
		vl := &VisitLog{u.String(), title, description}

		logQ <- vl

		doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
			val, _ := s.Attr("href")
			//log.Debug("doc.find: val=%s\n", val)
			u, err := url.Parse(val)
			if err != nil {
				log.Debug("parse failed\n")
				return
			}
			//log.Debug("q len is %di\n", len(q))
			ss := u.String()

			if i := strings.Index(ss, "#"); i != -1 {
				ss = ss[0:i]
			}

			in <- &Command{cmd.Url, ss, unknown}
			//f.back <- PageInfo{*u, title, description}

		})

		//zt.sender.Send(string(content), 0)
		//log.Debug("resp:%s\n",content)
		resp.Body.Close()
		time.Sleep(time.Millisecond)
	}

	wg.Done()
	log.Debug("after wg.Done")

}
