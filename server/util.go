/**
* Copyright 2023 tldb Author. All Rights Reserved.
* email: donnie4w@gmail.com
* github.com/donnie4w/tlnet
* tlnet websocket test demo
**/
package server

import (
	"crypto/md5"
	"encoding/hex"
	htmlTpl "html/template"
	"runtime/debug"
	"strings"
	"time"

	"github.com/donnie4w/simplelog/logging"
	"github.com/donnie4w/tlnet"
)

func htmlTplByPath(path string, data any, hc *tlnet.HttpContext) {
	if tp, err := htmlTpl.ParseFiles(path); err == nil {
		tp.Execute(hc.Writer(), data)
	} else {
		logging.Error(err)
	}
}

func TimeNow() (_r string) {
	_r = time.Now().Format("2006-01-02 15:04:05")
	return
}

func MyRecover() {
	if err := recover(); err != nil {
		logging.Error(string(debug.Stack()))
	}
}

func MD5(s string) string {
	m := md5.New()
	m.Write([]byte(s))
	return strings.ToUpper(hex.EncodeToString(m.Sum(nil)))
}
