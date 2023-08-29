/**
* Copyright 2023 tldb Author. All Rights Reserved.
* email: donnie4w@gmail.com
* github.com/donnie4w/tlnet
* tlnet websocket test demo
**/
package main

import (
	"flag"
	"tlnetIm/server"
)

func main() {
	addr := flag.String("addr", ":8009", "addr")
	port := flag.Int("port", 7000, "addr")
	server.UseRobot = flag.Bool("robot", false, "use robot")
	flag.Parse()
	server.TlnetStart(*addr, *port)
}
