/**
* Copyright 2023 tldb Author. All Rights Reserved.
* email: donnie4w@gmail.com
* github.com/donnie4w/tlnet
* tlnet websocket test demo
**/
package server

import (
	"fmt"

	"github.com/donnie4w/tlorm-go/orm"
)

func initOrm(port int) {
	err := orm.RegisterDefaultResource(true, fmt.Sprint("127.0.0.1:", port), "mycli=123")
	if err != nil {
		panic(fmt.Sprint("error:", err))
	}
	//tldb orm 建表
	orm.Create[ImUser]()
	orm.Create[ImMessage]()
	orm.Create[ImLog]()
}

type ImUser struct {
	Id    int64  // tlorm-go 要求每个表的结构体必须有一个Id int64 接收tldb服务器主键的赋值
	Name  string `idx:"1"` //Name字段添加索引
	Pwd   string
	Icon  string
	Label string
	Time  string
}

type ImLog struct {
	Id     int64
	UserId int64  `idx:"1"` //UserId字段添加索引
	Room   string `idx`     //Room字段添加索引  `idx` 与`idx:"1"` 效果一样
	Time   string
}

type ImMessage struct {
	Id      int64
	UserId  int64
	Content string
	Room    string `idx:"1"`
	Time    string
}
