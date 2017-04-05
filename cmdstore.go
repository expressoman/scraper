package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	//cmdStoreAddr string = "mongodb://superAdmin:mstoobad@127.0.0.1:27017/%s?authSource=admin"
	cmdStoreAddr string = "mongodb://127.0.0.1/%s"
	dbName       string = "scraper"
	colName      string = "Command"
)
const (
	unknown = iota
	inQueue
	gotten
	failed
)

type Command struct {
	Source   string
	Url      string
	Accessed int
	Deep     int
}

type VisitLog struct {
	Url         string
	Title       string
	Description string
}

type cmdStore struct {
	session *mgo.Session
	db      *mgo.Database
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
	c := session.DB(dbName).C(colName)
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

	cms.session = session

	cms.db = session.DB(dbName)
	return cms
}

func (cms *cmdStore) addCommand(cmd *Command) error {
	c := cms.db.C(colName)
	err := c.Insert(cmd)
	//err:=c.Insert(&Command{Source:"test",Url:"tttt",Accessed:111})
	log.Debug("cmd=", cmd)
	//err:=c.Insert(&Command{source:cmd.source,url:cmd.url,accessed:cmd.accessed})
	if err != nil {
		log.Debug("cmd insert err:", err)
	}
	return err
}

func (cms *cmdStore) nextCommand() (*Command, error) {
	c := cms.db.C(colName)
	//query:=fmt.Sprintf("{accessed:%d}",unknown)
	//log.Debug("query=",query)
	var cmd Command

	err := c.Find(bson.M{"accessed": unknown}).One(&cmd)
	if err != nil {
		log.Debug("find err=", err)
	}

	return &cmd, err

}
func (cms *cmdStore) updateCommand(url string) error {
	c := cms.db.C(colName)
	err := c.Update(bson.M{"url": url}, bson.M{"$set": bson.M{"accessed": inQueue}})
	if err != nil {
		log.Fatal("cmd update failed, err=", err)
	}
	return err
}

func (cms *cmdStore) visitLog(vl *VisitLog) error {
	c := cms.db.C("log")
	err := c.Insert(vl)
	if err != nil {
		log.Debug("visitLog insert err:", err)
	}
	return err
}

func (cms *cmdStore) close() {
	cms.session.Close()
}

/*
func main() {

	session, err := mgo.Dial("mongodb://superAdmin:mstoobad@127.0.0.1:27017/test?authSource=admin")
	if err != nil {
		panic(err)
	}
	defer session.Close()

	// Optional. Switch the session to a monotonic behavior.
	session.SetMode(mgo.Monotonic, true)

	c := session.DB("test").C("people")
	err = c.Insert(&Person{"Ale", "+55 53 8116 9639"},
		&Person{"Cla", "+55 53 8402 8510"})
	if err != nil {
		log.Fatal(err)
	}

	result := Person{}
	err = c.Find(bson.M{"name": "Ale"}).One(&result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Debug("Phone:", result.Phone)
}*/
