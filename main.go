package main

import (
	"flag"
	"fmt"
	zmq "github.com/pebbe/zmq4"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"
	"sync"
	_ "time"
)

var (
	urls_file  = flag.String("urls_file", "urls.txt", "seed URL file")
	hander_num = flag.Int("handler_num", 4, "handler number")
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
	zt.sender.Connect("tcp://localhost:5557")

	zt.receiver, _ = zmq.NewSocket(zmq.PULL)
	//defer receiver.Close()
	zt.receiver.Connect("tcp://localhost:5558")
	return zt
}
type (zt *zmqTool)func close(){
	zt.receiver.Close()
	zt.sender.Close()
}

func main() {
	flag.Parse()

	zt := newZmqTool()
	defer zt.close()
	
	v := runtime.NumCPU()
	fmt.Println("NumCPU=", v)
	if *hander_num > v {
		*hander_num = v
	}
	q := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < *hander_num; i++ {
		wg.Add(1)
		go handler(q, zt, &wg)
	}

	urls, err := ioutil.ReadFile(*urls_file)
	if err != nil {
		fmt.Println("read urls.txt error, ", err)
		return
	}

	strs := strings.Split(string(urls), "\n")

	for _, str := range strs {
		fmt.Println("before q<-str, str=", str)
		if len(str) > 0 {
			q <- str
		}
	}
	for i := 0; i < *hander_num; i++ {
		q <- ""
	}

	wg.Wait()

	fmt.Println("end of main")
}

func handler(q chan string, zt *zmqTool, wg *sync.WaitGroup) {
	for {
		fmt.Println("before s:=<-q")
		s := <-q
		if len(s) == 0 {
			fmt.Println(" end of urls ")
			break
		}
		fmt.Println("after s:=<-q, s=", s)

		zt.sender.Send("will visit " + s,0)

		client := http.Client{}

		resp, err := client.Get(s)
		//defer resp.Body.Close()

		if err != nil {
			fmt.Printf("error message: %s\n", err)
			continue
		}
		//content, err := ioutil.ReadAll(resp.Body)

		//zt.sender.Send(string(content), 0)
		//fmt.Printf("resp:%s\n",content)
		resp.Body.Close()
	}

	wg.Done()
	fmt.Println("after wg.Done")

}
