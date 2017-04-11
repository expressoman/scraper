package info

import "testing"

var tvsi = &VisitStatInfo{
	InfoType : 0,
	Msg: VisitStat{
		Req: 130,
		Success: 100,
		Failed: 30,
	},
}

var tvsiStr=`{"InfoType":0,"Msg":{"Req":130,"Success":100,"Failed":30}}`


var tvli = &VisitLogInfo{
	InfoType : 1,
	Msg: VisitLog{
		Url: "http://github.com",
		Title: "github",
		Description:"host for world",
	},
}

var tvliStr=`{"InfoType":1,"Msg":{"Url":"http://github.com","Title":"github","Description":"host for world"}}`

func TestMarshal(t *testing.T){
	mstr,err:=Marshal(tvli)
	
	t.Log("   mstr=",mstr)
	t.Log("tvliStr=",tvliStr)

	if err != nil{
		t.Fail()
	}
	if mstr != tvliStr {
		
		t.Log("do not match!")
		t.Fail()
	}
	
	mstr,err = Marshal(tvsi)
	
	t.Log("   mstr=",mstr)
	t.Log("tvsiStr=",tvsiStr)

	if err != nil{
		t.Fail()
	}
	if mstr != tvsiStr {
		
		t.Log("do not match!")
		t.Fail()
	}
}

func BenchmarkMarshal(b *testing.B){
	for i:=0;i<b.N;i++{
		_,_=Marshal(tvli)
	}

}
func TestUnmarshal(t *testing.T){
	it,msg,err:=Unmarshal(tvliStr)
	
	t.Log("tvliStr=",tvliStr)
	if err != nil {
		t.Log("fail to unmarshal")
	}

	var vl *VisitLog
	vl = msg.(*VisitLog)
	if it != Log || 
		vl.Url != "http://github.com" ||
		vl.Title != "github" ||
		vl.Description != "host for world" {

		t.Log("do not match!")
		t.Fail()
	}
	
	it,msg,err=Unmarshal(tvsiStr)
	
	t.Log("tvsiStr=",tvsiStr)
	if err != nil {
		t.Log("fail to unmarshal")
	}

	var vs *VisitStat
	vs = msg.(*VisitStat)
	if it != Stat || 
		vs.Req != 130 ||
		vs.Success != 100 ||
		vs.Failed != 30 {

		t.Log("do not match!")
		t.Fail()
	}

}

func BenchmarkUnmarshal(b *testing.B){
	for i:=0;i<b.N;i++{
		Unmarshal(tvliStr)
	}

}

