package lib

import (
	"fmt"
	"log"
	"runtime"
)

const (
	color_red = uint8(iota + 91)
	color_green
	color_yellow
	color_blue
	color_magenta //洋红
	info = "[INFO]"
	trac = "[TRAC]"
	erro = "[ERRO]"
	warn = "[WARN]"
	succ = "[SUCC]"
)

func GetParams()(string,string,int){
	pc, file, line, _ := runtime.Caller(2)
	funcname := runtime.FuncForPC(pc).Name()
	return file, funcname, line
}

func XLogInfo(objs...interface{}){
	text := ""
	_, name, line := GetParams()
	for i, obj := range objs {
		if i == 0 {
			text += fmt.Sprintf("%v", obj)
		} else {
			text += fmt.Sprintf(", %v", obj)
		}
	}
	log.Printf("\x1b[%dmINFO: %s:%d %s\x1b[0m", color_yellow, name, line, text)
}

func XLogErr(objs...interface{}){
	text := ""
	_, name, line := GetParams()
	for i, obj := range objs {
		if i == 0 {
			text += fmt.Sprintf("%v", obj)
		} else {
			text += fmt.Sprintf(", %v", obj)
		}
	}
	log.Printf("\x1b[%dmERR: %s:%d %s\x1b[0m", color_red, name, line, text)
}
