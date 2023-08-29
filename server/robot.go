package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// 定义一个机器人用户
var robot = &ImUser{Id: 1<<60 + 1, Name: "robot", Label: "我是机器人, @robot 和我聊天", Icon: "/img/30.png"}

func getRobotAck(msg string) (_r string) {
	urlstr := fmt.Sprint("http://127.0.0.1:8999/q?q=", msg)
	if resp, err := http.Get(urlstr); err == nil {
		defer resp.Body.Close()
		var buf bytes.Buffer
		io.Copy(&buf, resp.Body)
		if buf.Bytes() != nil {
			_r = string(buf.Bytes())
		}
	}
	return
}
