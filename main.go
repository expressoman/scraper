package main

import (
	_ "encoding/json"
	"flag"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	log "github.com/Sirupsen/logrus"
	"github.com/djimenez/iconv-go"
	info "github.com/moris351/scraper/info"
	zmq "github.com/pebbe/zmq4"
	_ "golang.org/x/net/html"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"io"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	logstatPort  = ":5557"
	cmdPort      = ":5558"
	maxVisitDeep = 5
)
const (
	//cmdStoreAddr string = "mongodb://superAdmin:mstoobad@127.0.0.1:27017/%s?authSource=admin"
	cmdStoreAddr string = "mongodb://127.0.0.1/%s"
	dbName       string = "scraper"
	cmdCol       string = "Command"
	visitCol     string = "Visit"
	hostsCol     string = "Hosts"
)
const (
	unknown = iota
	inQueue
	gotten
	failed
)

const (
	cmdQueueLen  = 100
	hostBatchNum = 30
)
var (
	urls_file      = flag.String("urls_file", "urls.txt", "seed URL file")
	hander_num     = flag.Int("handler_num", 4, "handler number")
	sconsole_addr  = flag.String("sconsole address", "localhost", "sconsole ip address")
	debug_level    = flag.String("debug_level", "info", "debug level: debug, info, warning, error")
	max_visit_deep = flag.Int("max_visit_deep", maxVisitDeep, "max visit deep")
	version        = flag.Bool("version", false, "version info")
)

var (
	inQue  chan *info.Command      = make(chan *info.Command, 10*cmdQueueLen)
	outQue chan *info.Command      = make(chan *info.Command, cmdQueueLen)
	preOutQue chan *info.Command   = make(chan *info.Command, cmdQueueLen)
	logQue chan *info.VisitLogInfo = make(chan *info.VisitLogInfo, 1)

	reqQue chan interface{}        = make(chan interface{})
	errQue chan interface{}        = make(chan interface{})
	closeQue chan interface{}	=make(chan interface{})
	lb *logBot = nil
)

type zmqTool struct {
	receiver *zmq.Socket
	sender   *zmq.Socket
}

func newZmqTool() *zmqTool {
	zt := new(zmqTool)
	//var err error
	zt.sender, _ = zmq.NewSocket(zmq.PUB)
	//defer sender.Close()
	zt.sender.Bind("tcp://*" + logstatPort)
	//zt.sender.Bind("ipc://logstat.ipc")

	zt.receiver, _ = zmq.NewSocket(zmq.PULL)
	//defer receiver.Close()
	zt.receiver.Connect("tcp://" + *sconsole_addr + cmdPort)
	return zt
}
func (zt *zmqTool) close() {
	zt.receiver.Close()
	zt.sender.Close()
}

var (
	VerTag    string
	BuildTime string
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to this file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

func main() {
	flag.Parse()
	fmt.Println("Version Tag: " + VerTag)
	fmt.Println("Build Time: " + BuildTime)

	if *version {
		return
	}
	
	log.Info("scraper session begin")

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	lb = newLogBot()
	defer lb.close()

	//lb.Debug("test")

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
	go parseLog(cms,zt)
	for _, str := range strs {
		if len(str) > 0 {
			u, err := url.Parse(str)
			if err == nil {

				cmd := &info.Command{Source: "",
					Host:     rootHostname(u.Hostname()),
					Url:      u.String(),
					Accessed: unknown,
					Deep:     0}

				log.Debug("cmd=", cmd)
				inQue <- cmd
			}
		}
	}

	sigQue := make(chan os.Signal)
	//	signal.Notify(lb.ch, syscall.SIGUSR1,syscall.SIGINT,syscall.SIGTERM)
	signal.Notify(sigQue, syscall.SIGINT)
	go func (){
		<-sigQue
		log.Info("SIGINT received")
		closeQue<-nil
	}()


	go func() {
		log.Println(http.ListenAndServe(":6060", nil))
	}()

	wg.Wait()
	log.Debug("end of main")
}

func parseLog(cms *cmdStore, zt *zmqTool){
	var vsi info.VisitStatInfo
	vsi.InfoType = info.Stat

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select{
		case <-errQue:
			vsi.Msg.Failed++
			//log.Debug("vl:=<-log,vl=", vl)
		case <-ticker.C:
			b, err := info.Marshal(vsi)
			if err == nil {
				zt.sender.Send(b, 0)
			}
		case <-reqQue:
			vsi.Msg.Req++

		case vli := <-logQue:
			log.Debug("vli:=<-log,vli=", vli)
			vsi.Msg.Success++
			cms.visitLog(&vli.Msg)

			b, err := info.Marshal(vli)
			if err == nil {
				zt.sender.Send(b, 0)
			}
		}
	}
}
func parseQueue(cms *cmdStore) {

	var outCmd *info.Command
	//var err error
	parseQueueLoop:
	for {
		select{
		case <-closeQue:
			outQue<-&info.Command{Action:0}
			close(closeQue)
			log.Info("break parseQueueLoop")
			break parseQueueLoop
		default:
			//keep going
		}

		if outCmd == nil {
			if len(preOutQue) == 0{
				start:=time.Now()
				err := cms.fillPreCmdQue()
				if err != nil {
					//should be not found err ontinue            
				    log.Warnln("should not be found here")
				    incmd := <-inQue
				    //cms.updateCommand(incmd.Url)           
				    log.Debug("incmd=", incmd)
				    cms.addCommand(incmd)
				    outCmd = nil
				    continue
				}


				log.Debugf("cms.fillPreCmdQue last for %v", time.Since(start))
			}

			outCmd =<-preOutQue
		}
		log.Debugln("goroutine num=", runtime.NumGoroutine())
		select {
				
		case incmd := <-inQue:
			log.Debug("incmd=", incmd)
			//cms.updateCommand(incmd.Url)
			cms.addCommand(incmd)
		case outQue <- outCmd:
			log.Debug("outQue<-outCmd,outCmd=", outCmd)
			cms.updateCommand(outCmd.Url)
			reqQue<-nil
			outCmd = nil
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

	gid := GoID()

	handlerLoop:
	for {
		select {
		case <-closeQue:
			//log.WithFields(log.Fields{"gid":gid}).Info("break handlerLoop")
			lb.Info(gid,"break handlerLoop")
			break handlerLoop
		case cmd := <-outQue:
			log.Debugf("[G:%d]before s:=<-q", gid)
			if cmd != nil && cmd.Action==0 {
				log.Debug(" end of cmds ")
				break
			}
			parseCmd(gid,cmd)
		}
		time.Sleep(time.Millisecond)
	}
	wg.Done()
	log.Debug("after wg.Done")
}
func convUtf8(resp *http.Response) ( *goquery.Document,  error){
	
	v := resp.Header.Get("Content-Type")
	log.WithFields(log.Fields{"charset": v}).Debug("charset")

	cs := strings.Split(v, "=")
	vd := "gbk"

	if len(cs) == 2 {
		vd = strings.ToLower(cs[1])
	} 
	//else {
		//log.Errorf("wrong charset,%s,  can not process, continue",v)
		//return err
	//	vd="utf-8"
	//}
	// Convert the designated charset HTML to utf-8 encoded HTML.
	// `charset` being one of the charsets known by the iconv package.
	var doc *goquery.Document
	var utfBody *iconv.Reader
	var converter *iconv.Converter

	var err error
	
	defer func(){
		if converter != nil { converter.Close()}
	}()
	if( vd != "utf-8" ){
		utfBody,converter, err = NewReader(resp.Body, vd, "utf-8")
		if err != nil {
			// handler error
			//utfBody.Close()

			log.Error("iconv.NewReader return err=", err)
			return nil, err
		}
		
		doc, err = goquery.NewDocumentFromReader(utfBody)
	}else{

		doc, err = goquery.NewDocumentFromResponse(resp)
	}
	if err != nil {
		//log.Debug("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
		//utfBody.Close()
		
		log.Error("goquery.NewDocumentFromResponse err=", err)
		return nil, err
	}
	return doc, err
}

// NewReader is modified to close the converter
func NewReader(source io.Reader, fromEncoding string, toEncoding string) (*iconv.Reader, *iconv.Converter, error) {
	// create a converter
		converter, err := iconv.NewConverter(fromEncoding, toEncoding)

	if err == nil {
		reader := iconv.NewReaderFromConverter(source, converter)
		//converter.Close()
		return reader, converter,err
	}

	// return the error
	return nil,nil, err
}

func parseCmd(gid int, cmd *info.Command) error{

	var resp *http.Response
	//var utfBody *iconv.Reader
	var err error
	
	defer func(){
		if resp !=nil {
			resp.Body.Close()
		}
		/*if utfBody != nil {
			utfBody.Close()
		}*/
		if err != nil {
			errQue<-nil
		}
	}()
	
	log.Debug("after s:=<-q, s=", cmd.Url)

	u, err := url.Parse(cmd.Url)
	if err != nil {
		log.Errorln("url.Parse err, ", err)
		return err
	}
	log.WithFields(log.Fields{
		"url": u.String(),
	}).Info("processing")

	req, err := http.NewRequest("GET", u.String(), nil)
	req.Close = true
	client := http.Client{
		Timeout: time.Duration(15 * time.Second),
	}
	resp, err = client.Do(req)
	if err != nil {
		log.Printf("http.DefaultClient failed, err=", err)
		return err
	}

	doc, err := convUtf8(resp)
	if err != nil {
		return err
	}
	description, _ := doc.Find("head meta[name='description']").Attr("content")
	//log.Debugf("description=%s", description)

	title := doc.Find("html title").Text()

	log.WithFields(log.Fields{"title": title, "description": description}).Info("content received")

	vli := &info.VisitLogInfo{
		InfoType:info.Log,
		Msg:info.VisitLog{
			Url:u.String(),
			Title:title,
			Description:description,},
	}
	lb.Debug(gid,"vli=",vli)
	logQue <- vli

	if cmd.Deep >= *max_visit_deep {
		return nil
	}
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		val, _ := s.Attr("href")
		lb.Debug(gid,"doc.find: val=", val)
		u, err := u.Parse(val)
		if err != nil {
			log.Errorf("parse failed, href=%s, err=%v", val, err)
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
		inQue <- &info.Command{
			1,
			cmd.Url,
			rh,
			ss,
			unknown,
			deep}
	})


	return err

}

func GoID() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}
