package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	info "github.com/moris351/scraper/info"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"math/rand"
	"errors"
)

type cmdStore struct {
	session *mgo.Session
	db      *mgo.Database
	hosts   map[string]int
	cmdQue  chan info.Command
	sl      [hostBatchNum]string
}

func newCmdStore(dbName string) *cmdStore {
	cms := new(cmdStore)
	str := fmt.Sprintf(cmdStoreAddr, dbName)
	log.Debug(str)
	session, err := mgo.Dial(str)
	if err != nil {
		log.Fatal("connect mgdb failed, err=", err)
	}

	session.SetMode(mgo.Monotonic, true)
	c := session.DB(dbName).C(cmdCol)
	// Index
	index := mgo.Index{
		Key:        []string{"url"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(index)
	if err != nil {
		panic(err)
	}
	c = session.DB(dbName).C(hostsCol)
	// Index
	index1 := mgo.Index{
		Key:        []string{"hostname"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(index1)
	if err != nil {
		panic(err)
	}

	cms.session = session

	cms.db = session.DB(dbName)

	cms.hosts = make(map[string]int)

	cms.cmdQue = make(chan info.Command, cmdQueueLen)

	//cms.sl = make([]string,cmdQueueLen)

	c = cms.db.C(hostsCol)

	var results []info.Host
	c.Find(nil).All(&results)

	for _, h := range results {
		cms.hosts[h.Hostname] = h.UrlNum
	}

	/*u,err:= url.Parse(cmd.Url)
	if err != nil {
		log.Error("url parse error")
	}else{
		cms.hosts[u.Hostname()]++
	}*/

	return cms
}

func (cms *cmdStore) addCommand(cmd *info.Command) error {
	c := cms.db.C(cmdCol)
	err := c.Insert(cmd)
	//err:=c.Insert(&Command{Source:"test",Url:"tttt",Accessed:111})
	log.Debug("cmd=", cmd)
	//err:=c.Insert(&Command{source:cmd.source,url:cmd.url,accessed:cmd.accessed})
	if err != nil {
		log.Debug("cmd insert err:", err)
	}
	c = cms.db.C(hostsCol)

	//err = c.Find(Bson.M{"host":cmd.Host}).All(
	err = c.Insert(&info.Host{Hostname: cmd.Host, UrlNum: 0})

	cms.hosts[cmd.Host]++

	return err
}

func (cms *cmdStore) fillPreCmdQue() error {


	if len(preOutQue)>0 {
		log.Debug("fillPreCmdQue: still no need fill, return")
		return errors.New("no need")
	}

	i := 0
	var err error

	c := cms.db.C(cmdCol)
	var cmds []info.Command
	
	for h, _ := range cms.hosts {
		log.Debug("host :", h)
		cms.sl[i] = h
		i++
		if i == hostBatchNum {
			err = c.Find(bson.M{"accessed": unknown,
				"deep": bson.M{"$lt": *max_visit_deep},
				"host": bson.M{"$in": cms.sl}}).
				Limit(cmdQueueLen).
				All(&cmds)

			if err == nil || len(cmds) > 0 {
				break
			}
			log.Warnf("find err=%v, cms.sl=%v", err, cms.sl)
			i = 0
		}
	}

	if err !=nil  {	return err }
	if len(cmds)==0 { return errors.New("no more")}

	perm := rand.Perm(len(cmds))
	for _, v := range perm {
		preOutQue <-&cmds[v]
	}
	return nil
	
}
func (cms *cmdStore) updateCommand(url string) error {
	c := cms.db.C(cmdCol)
	err := c.Update(bson.M{"url": url}, bson.M{"$set": bson.M{"accessed": inQueue}})
	if err != nil {
		log.Fatal("cmd update failed, err=", err)
	}
	return err
}

func (cms *cmdStore) visitLog(vl *info.VisitLog) error {
	c := cms.db.C(visitCol)
	err := c.Insert(vl)
	if err != nil {
		log.Debug("visitLog insert err:", err)
	}

	log.Info("visitlog:", vl)

	return err
}

func (cms *cmdStore) close() {
	cms.session.Close()
}
