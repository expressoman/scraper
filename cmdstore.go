package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	info "github.com/moris351/scraper/info"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"math/rand"
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
	cmdQueueLen  = 1000
	hostBatchNum = 30
)

/*type Command struct {
	Source   string
	Host     string
	Url      string
	Accessed int
	Deep     int
}

type VisitStat struct {
	Req int
	Success int
	Failed	int
}
type VisitLog struct {
	Url         string
	Title       string
	Description string
}
*/
type cmdStore struct {
	session *mgo.Session
	db      *mgo.Database
	hosts   map[string]int
	cmdQue  chan info.Command
	sl      [cmdQueueLen]string
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

func (cms *cmdStore) nextCommand() (*info.Command, error) {
	c := cms.db.C(cmdCol)
	//query:=fmt.Sprintf("{accessed:%d}",unknown)
	//log.Debug("query=",query)
	var cmds []info.Command
	var hostname string
	var err error
	if len(cms.hosts) == 0 {
		return nil, nil
	}

	if len(cms.cmdQue) > 0 {
		k := <-cms.cmdQue
		log.Printf("cms.cmdQue k=%v len=%d", k, len(cms.cmdQue))

		return &k, nil
	}

	//sl := make([]string,cmdQueueLen)
	//scrProf.Add(&sl,1)
	i := 0
	for h, _ := range cms.hosts {
		hostname = h
		log.Debug("host :", hostname)
		cms.sl[i] = hostname
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
			log.Errorf("find err=%v, cms.sl=%v", err, cms.sl)
			i = 0
		}
	}

	perm := rand.Perm(cmdQueueLen)
	for _, v := range perm {
		log.Infof("cms.cmdQue[%d]=%v", v, cmds[v])

		cms.cmdQue <- cmds[v]
	}
	v := <-cms.cmdQue
	//scrProf.Remove(&sl)

	return &v, nil

	//return cmds, err

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
