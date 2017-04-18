package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	_ "os"
)

type Server struct {
	Ip   string `json:"ip"`
	Port string `json:"port"`
}

type config struct {
	LogstatPort  string   `json:"logstat_port"`
	CmdPort      string   `json:"cmd_port"`
	MaxVisitDeep int      `json:"max_visit_deep"`
	More         string   `json:"more"`
	CanTry       bool     `json:"can_try"`
	Servers      []Server `json:"servers"`
}

func (c *config) init() error {

	fcont, err := ioutil.ReadFile("./config.json")
	if err != nil {
		fmt.Printf("File error: %v\n", err)
		return err
	}
	fmt.Printf("%s\n", string(fcont))

	//m := new(Dispatch)
	//var m interface{}
	//var jsontype jsonobject
	//var cfg config
	//More default value
	c.More = "I am default"

	err = json.Unmarshal(fcont, c)
	if err != nil {
		fmt.Printf("Unmarshal err=%v", err)
		return err
	}
	fmt.Printf("Results: %v\n", c)
	fmt.Printf("c.More=%s\n", c.More)
	return nil
}
