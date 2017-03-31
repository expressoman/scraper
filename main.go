package main

import(
	"fmt"
	"io/ioutil"
	"net/http"
	_ "time"
	"sync"
	"strings"
	"flag"
	"runtime"
)
var(
	urls_file        = flag.String("urls_file", "urls.txt", "seed URL file")
	hander_num       = flag.Int("handler_num", 4, "handler number")
)

func main() {
	flag.Parse()
	v:=runtime.NumCPU()
	fmt.Println("NumCPU=",v)
	if(*hander_num>v){
		*hander_num=v
	}
	q:=make(chan string)
	var wg sync.WaitGroup
	for i:=0;i<*hander_num;i++{
		wg.Add(1)
		go handler(q,&wg)
	}

	urls,err:=ioutil.ReadFile(*urls_file)
	if(err!=nil){
		fmt.Println("read urls.txt error, ",err)
		return
	}

	strs:=strings.Split(string(urls),"\n")


	for _,str:=range strs{
		fmt.Println("before q<-str, str=",str)
		if len(str)>0 {
			q<-str
		}
	}
	for i:=0;i<*hander_num;i++{
		q<-""
	}

	wg.Wait()

	fmt.Println("end of main")
}

func handler(q chan string, wg *sync.WaitGroup){
	for{
		fmt.Println("before s:=<-q")
		s:=<-q
		if len(s)==0 { 
			fmt.Println(" end of urls ")
			break
		}
		fmt.Println("after s:=<-q, s=",s)
		client := http.Client{}
		resp, err := client.Get(s)
		//defer resp.Body.Close()

		if err != nil {
			fmt.Printf("error message: %s\n", err)
			continue 
		}
		//content,err:=ioutil.ReadAll(resp.Body)
		//fmt.Printf("resp:%s\n",content)
		resp.Body.Close()
	}

	wg.Done()
	fmt.Println("after wg.Done")
	
}
