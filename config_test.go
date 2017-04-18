package main

import "testing"

func TestConfig(t *testing.T) {
	c := &config{}
	err := c.init()
	if err != nil {
		t.Errorf("err=%v", err)
		return
	}

	t.Logf("config: %v", c)

}
