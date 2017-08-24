package info

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
)

type Command struct {
	Action 	 int
	Source   string
	Host     string
	Url      string
	Accessed int
	Deep     int
}

type VisitStat struct {
	Req     int
	Success int
	Failed  int
}
type VisitLog struct {
	Url         string
	Title       string
	Description string
}

type Host struct {
	Hostname string
	UrlNum   int
}

type IT int

const (
	Stat IT = iota
	Log  IT = iota
)

type VisitStatInfo struct {
	InfoType IT
	Msg      VisitStat
}
type VisitLogInfo struct {
	InfoType IT
	Msg      VisitLog
}

func Marshal(i interface{}) (string, error) {
	b, err := json.Marshal(i)
	return string(b), err
}

type vsi struct {
	InfoType IT
	Msg      json.RawMessage
}

func Unmarshal(str string) (IT, interface{}, error) {
	//log.Infoln("str=",str)
	b := []byte(str)
	var i vsi
	err := json.Unmarshal(b, &i)
	if err != nil {
		log.Error("json.Unmarshal err,", err)
	}

	//log.Infof("i.Msg=%v, len=%d", i.Msg,len(i.Msg))
	var dst interface{}

	switch i.InfoType {
	case Stat:
		dst = new(VisitStat)
	case Log:
		dst = new(VisitLog)
	}

	err = json.Unmarshal(i.Msg, dst)

	if err != nil {
		log.Error("Info Unmarshal error, ", err)
	}

	return i.InfoType, dst, err
}

func (c *VisitLog) String() string {
	return c.Url + "==>" + c.Title + "==>" + c.Description
}
