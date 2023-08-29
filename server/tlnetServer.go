/**
* Copyright 2023 tldb Author. All Rights Reserved.
* email: donnie4w@gmail.com
* http framework : github.com/donnie4w/tlnet
* mq & database : github.com/donnie4w/tldb
* tlnet websocket test demo
**/
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	. "github.com/donnie4w/gofer/hashmap"
	"github.com/donnie4w/simplelog/logging"
	"github.com/donnie4w/tlmq-go/cli"
	. "github.com/donnie4w/tlmq-go/stub"
	"github.com/donnie4w/tlnet"
	"github.com/donnie4w/tlorm-go/orm"
)

// 该项目只是demo，主要是演示tlnet和tldb 的部分使用，实际消息系统的构建应该根据项目本身的设计作调整
type ATYPE string

var UseRobot *bool

const (
	FRIEND ATYPE = "friend"
	MSG    ATYPE = "msg"
	NOPASS ATYPE = "nopass"
	LOGIN  ATYPE = "login"
	LOGOUT ATYPE = "logout"
)

func Init(dbport int) {
	_, er1 := logging.SetGzipOn(true).SetRollingFile("", "im.log", 1, logging.MB)
	if er1 != nil {
		panic(fmt.Sprint("error:", er1))
	}
	initOrm(dbport)
	immq.init()
}

func TlnetStart(addr string, port int) {
	Init(port)
	tlnet := tlnet.NewTlnet()
	tlnet.Handle("/register", register)
	tlnet.HandleStaticWithFilter("/", "./html", notFoundFilter(), nil)
	tlnet.HandleWebSocketBindConfig("/ws", wshandlerFunc, wsWebsocketConfig())
	logging.Info("tlnet im demo start:", addr)
	if err := tlnet.HttpStart(addr); err != nil {
		logging.Error("TlnetStart failed:", err)
	}
}

// 拦截器，如果404找不到页面，重定向到 根路径/
func notFoundFilter() *tlnet.Filter {
	f := tlnet.NewFilter()
	f.AddPageNotFoundIntercept(func(hc *tlnet.HttpContext) bool {
		hc.Redirect("/")
		return true
	})
	return f
}

var roomap = NewMapL[string, *MapL[*tlnet.Websocket, *ImUser]]()
var wsmap = NewMapL[*tlnet.Websocket, string]()
var wamap = NewMap[int64, *MapL[*tlnet.Websocket, int8]]()
var mux = &sync.Mutex{}

func store(ws *tlnet.Websocket, u *ImUser, room string) {
	mux.Lock()
	defer mux.Unlock()
	wsmap.Put(ws, room)
	var m *MapL[*tlnet.Websocket, *ImUser]
	var ok bool
	if m, ok = roomap.Get(room); !ok {
		m = NewMapL[*tlnet.Websocket, *ImUser]()
	}
	m.Put(ws, u)
	roomap.Put(room, m)
	var m1 *MapL[*tlnet.Websocket, int8]
	if m1, ok = wamap.Get(u.Id); !ok {
		m1 = NewMapL[*tlnet.Websocket, int8]()
	}
	m1.Put(ws, 0)
	wamap.Put(u.Id, m1)
}

func delws(ws *tlnet.Websocket) {
	mux.Lock()
	defer mux.Unlock()
	if r, ok := wsmap.Get(ws); ok {
		if m, ok := roomap.Get(r); ok {
			if u, ok := m.Get(ws); ok {
				if _wsm, ok := wamap.Get(u.Id); ok {
					_wsm.Del(ws)
					if _wsm.Len() == 0 {
						wamap.Del(u.Id)
					}
				}
				m.Del(ws)
			}
			if m.Len() == 0 {
				roomap.Del(r)
			}
		}
		wsmap.Del(ws)
	}
}

func getIu(room string, ws *tlnet.Websocket) (iu *ImUser, ok bool) {
	if m, b := roomap.Get(room); b {
		iu, ok = m.Get(ws)
	}
	return
}

type wsack struct {
	ATYPE    ATYPE
	MSG      string
	USERNAME string
	ICON     string
	LABEL    string
	TIME     string
	ROOM     string
}

func (this wsack) toJson() (_r string) {
	if v, err := json.Marshal(this); err == nil {
		_r = string(v)
	}
	return
}

var nodeId = time.Now().UnixNano()

type mqws struct {
	NodeId int64
	Room   string
	UserId int64
	Wa     []*wsack
}

func register(hc *tlnet.HttpContext) {
	username, password, icon := hc.PostParamTrimSpace("username"), hc.PostParamTrimSpace("password"), hc.PostParamTrimSpace("logo")
	if username != "" && password != "" && icon != "" {
		if u, _ := orm.SelectByIdx[ImUser]("Name", username); u == nil {
			orm.Insert(&ImUser{Name: username, Pwd: MD5(password), Icon: icon, Time: TimeNow()})
			hc.RedirectWithStatus("/", http.StatusFound)
		} else {
			htmlTplByPath("./html/ack.html", "Note:User name already exists!", hc)
		}
	} else {
		htmlTplByPath("./html/ack.html", "Note:incomplete information", hc)
	}
}

// 定义websocket发生错误或打开连接时的操作函数
func wsWebsocketConfig() (wc *tlnet.WebsocketConfig) {
	wc = &tlnet.WebsocketConfig{}
	//websocket断开时，触发OnError。删除wsmap中的连接
	wc.OnError = func(self *tlnet.Websocket) {
		defer MyRecover()
		if r, ok := wsmap.Get(self); ok {
			if u, ok := getIu(r, self); ok {
				//掉线通知
				broadcast(&wsack{ATYPE: LOGOUT, USERNAME: u.Name}, nil, r, true, true)
			}
		}
		delws(self)
	}
	//wc.OnOpen 用在连接成功时调用
	return
}

// websocket 接收到数据后的操作
func wshandlerFunc(hc *tlnet.HttpContext) {
	defer MyRecover()
	var wa wsack
	if err := json.Unmarshal(hc.WS.Read(), &wa); err == nil { //hc.WS.Read() 读取websocket接收的数据
		parse(wa, hc.WS) //解析并处理信息
	}
}

func parse(wa wsack, ws *tlnet.Websocket) {
	room, ok := wsmap.Get(ws)
	if !ok {
		if wa.ATYPE == LOGIN {
			if iu, ok := getUserInfo(wa.MSG); ok {
				room = strings.TrimSpace(wa.ROOM)
				store(ws, iu, room)
				//记录登录日志
				orm.Insert(&ImLog{UserId: iu.Id, Room: room, Time: TimeNow()})
				ws.Send(wsack{ATYPE: wa.ATYPE, USERNAME: iu.Name, ICON: iu.Icon, TIME: TimeNow(), ROOM: room}.toJson())
				immq.PubId(room, iu.Id)
				//返回好友列表
				if *UseRobot {
					ws.Send(wsack{ATYPE: FRIEND, USERNAME: robot.Name, ICON: robot.Icon, LABEL: robot.Label}.toJson())
				}
				broadcastToSelf(&wsack{ATYPE: FRIEND}, ws, room)
				//通知好友
				broadcast(&wsack{ATYPE: FRIEND, USERNAME: iu.Name, TIME: TimeNow(), ICON: iu.Icon}, ws, room, true, true)
				//返回聊天室 最新N条数据
				if id, _ := orm.SelectIdByIdx[ImMessage]("Room", room); id > 0 {
					startid := id - 20
					if startid < 0 {
						startid = 0
					}
					if ims, _ := orm.SelectByIdxLimit[ImMessage](startid, 21, "Room", room); ims != nil {
						for _, im := range ims {
							var u *ImUser
							if im.UserId > 1<<60 {
								u = robot
							} else {
								u, _ = orm.SelectById[ImUser](im.UserId)
							}
							if u != nil {
								ws.Send(wsack{ATYPE: MSG, USERNAME: u.Name, ICON: u.Icon, MSG: im.Content, TIME: im.Time}.toJson())
							}
						}
					}
				}
			} else {
				ws.Send(wsack{ATYPE: NOPASS}.toJson())
			}
		}
	} else if wa.ATYPE == MSG {
		iu, _ := getIu(room, ws)
		t := TimeNow()
		//保存聊天信息
		if _, err := orm.Insert(&ImMessage{UserId: iu.Id, Content: wa.MSG, Time: t, Room: room}); err == nil {
			//发送聊天数据
			broadcast(&wsack{ATYPE: MSG, USERNAME: iu.Name, MSG: wa.MSG, TIME: t, ICON: iu.Icon}, nil, room, true, false)
			if strings.Contains(wa.MSG, "@robot") && *UseRobot {
				msg := strings.ReplaceAll(strings.ReplaceAll(wa.MSG, "@robot", ""), " ", "")
				if ackmsg := getRobotAck(msg); ackmsg != "" {
					ackmsg = fmt.Sprint("@", iu.Name, " ", ackmsg)
					if _, err := orm.Insert(&ImMessage{UserId: robot.Id, Content: ackmsg, Time: t, Room: room}); err == nil {
						broadcast(&wsack{ATYPE: MSG, USERNAME: robot.Name, MSG: ackmsg, TIME: t, ICON: robot.Icon}, nil, room, true, false)
					}
				}
			}
		}
	}
}
func broadcast(ack *wsack, nows *tlnet.Websocket, room string, ispub bool, mem bool) {
	s := ack.toJson()
	if m, ok := roomap.Get(room); ok {
		m.Range(func(k *tlnet.Websocket, _ *ImUser) bool {
			if nows != nil && k == nows {
				return true
			}
			k.Send(s)
			return true
		})
	}
	if ispub {
		if mem {
			immq.PubMem(nodeId, room, ack)
		} else {
			immq.Pub(nodeId, room, ack)
		}
	}
}
func broadcastToSelf(ack *wsack, ws *tlnet.Websocket, room string) {
	if m, ok := roomap.Get(room); ok {
		m.Range(func(_ *tlnet.Websocket, vu *ImUser) bool {
			ack.USERNAME, ack.ICON, ack.LABEL = vu.Name, vu.Icon, vu.Label
			ws.Send(ack.toJson())
			return true
		})
	}
}
func getUserInfo(auth string) (_r *ImUser, ok bool) {
	if ss := strings.Split(auth, "="); len(ss) == 2 {
		if u, _ := orm.SelectByIdx[ImUser]("Name", ss[0]); u != nil {
			if ok = u.Pwd == MD5(ss[1]); ok {
				_r = u
			}
		}
	}
	return
}

var immq = &ImMq{}

type ImMq struct {
	mq cli.MqClient //tldb mq 提供的简易客户端
}

func (this *ImMq) init() {
	this.mq = cli.NewMqClient("ws://127.0.0.1:5000", "mymq=123") //mq服务器地址与用户名密码
	if err := this.mq.Connect(); err != nil {                    //mq.Connect() 连接服务器
		panic("mq connect err:" + err.Error())
	}
	this.mq.MergeOn(1)              //设置服务器信息聚合发送到客户端，1表示数据包大小上限为1MB
	this.mq.Sub("immsg")            //订阅topic：immsg
	this.mq.Sub("id")               //订阅 topic：id
	this.mq.Sub(fmt.Sprint(nodeId)) //订阅本节点信息
	//处理订阅信息，接收发布函数PubMem()发送的数据,不存储信息
	this.mq.PubMemHandler(func(jmb *JMqBean) {
		defer MyRecover()
		var ms mqws
		json.Unmarshal([]byte(jmb.Msg), &ms)
		switch jmb.Topic {
		case "immsg":
			if ms.NodeId != nodeId {
				broadcast(ms.Wa[0], nil, ms.Room, false, false)
			}
		case "id":
			if m, ok := roomap.Get(ms.Room); ok {
				wss := make([]*wsack, 0)
				m.Range(func(_ *tlnet.Websocket, vu *ImUser) bool {
					if ms.UserId != vu.Id {
						wss = append(wss, &wsack{ATYPE: FRIEND, USERNAME: vu.Name, ICON: vu.Icon, LABEL: vu.Label})
					}
					return true
				})
				immq.PubInfo(ms.NodeId, ms.UserId, ms.Room, wss)
			}
		case fmt.Sprint(nodeId):
			if k, ok := wamap.Get(ms.UserId); ok {
				for _, v := range ms.Wa {
					k.Range(func(w *tlnet.Websocket, _ int8) bool {
						w.Send(v.toJson())
						return true
					})
				}
			}
		}
	})
	//处理订阅信息，这里使用json格式，接收发布函数PubJson()发送的数据，也可以使用 PubByteHandler()对应PubByte()
	this.mq.PubJsonHandler(func(jmb *JMqBean) {
		defer MyRecover()
		var ms mqws
		json.Unmarshal([]byte(jmb.Msg), &ms)
		switch jmb.Topic {
		case "immsg":
			if ms.NodeId != nodeId {
				broadcast(ms.Wa[0], nil, ms.Room, false, false)
			}
		}
	})
}

func (this *ImMq) Pub(id int64, room string, wa *wsack) {
	if bs, err := json.Marshal(&mqws{NodeId: id, Room: room, Wa: []*wsack{wa}}); err == nil {
		this.mq.PubJson("immsg", string(bs)) // PubJson 发布订阅信息
	}
}
func (this *ImMq) PubMem(id int64, room string, wa *wsack) {
	if bs, err := json.Marshal(&mqws{NodeId: id, Room: room, Wa: []*wsack{wa}}); err == nil {
		this.mq.PubMem("immsg", string(bs)) // PubMem 发布订阅信息,不存储消息
	}
}
func (this *ImMq) PubId(room string, userid int64) {
	if bs, err := json.Marshal(&mqws{NodeId: nodeId, Room: room, UserId: userid}); err == nil {
		this.mq.PubMem("id", string(bs)) // PubMem 发布订阅信息,不存储消息
	}
}
func (this *ImMq) PubInfo(nodeid, userid int64, room string, wss []*wsack) {
	if bs, err := json.Marshal(&mqws{NodeId: nodeid, Room: room, UserId: userid, Wa: wss}); err == nil {
		this.mq.PubMem(fmt.Sprint(nodeid), string(bs)) // PubMem 发布订阅信息,不存储消息
	}
}
