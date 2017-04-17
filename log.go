package main

import (
	log "github.com/Sirupsen/logrus"
	"io"
	"fmt"
	"os"
	"time"
    "syscall"
    "os/signal"

)

type logBot struct{
	file *os.File
	ch chan os.Signal
}
func newLogBot() *logBot{
	// Log as JSON instead of the default ASCII formatter.
	//log.SetFormatter(&log.JSONFormatter{})
	log.SetFormatter(&log.TextFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	lb := new(logBot)

	var err error
	lb.file, err = os.OpenFile("scraper.log", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err == nil {
		//	log.SetOutput(file)
		log.SetOutput(io.MultiWriter(lb.file, os.Stdout))

	} else {
		log.Fatal("Failed to log to file, using default stderr")
	}

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

	lb.ch = make( chan os.Signal)
//	signal.Notify(lb.ch, syscall.SIGUSR1,syscall.SIGINT,syscall.SIGTERM)
	signal.Notify(lb.ch, syscall.SIGUSR1)

	go func (){
		s:=<-lb.ch
		log.Printf("signal : %v",s)
		lb.reset()
	}()
	return lb
}

func (lb *logBot) close(){
	lb.file.Close()
}

func (lb *logBot) reset(){
	lb.file.Close()
	time.Sleep(1200*time.Millisecond)

	var err error
	lb.file, err = os.OpenFile("scraper.log", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err == nil {
		//	log.SetOutput(file)
		log.SetOutput(io.MultiWriter(lb.file, os.Stdout))

	} else {
		log.Fatal("Failed to log to file, using default stderr")
	}
}


