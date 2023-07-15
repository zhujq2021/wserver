package main

import (
//	"io"
	"log"
//	"net"
	"net/http"
	"github.com/gorilla/websocket"
)

const port = "80"

var upgrader = websocket.Upgrader{}

func defHandler(w http.ResponseWriter, r *http.Request) {
  conn, err := upgrader.Upgrade(w, r, nil)
  if err != nil {
        log.Println(err)
        return
    }
  defer conn.Close()
	for {
        // 读取消息
        messageType, p, err := conn.ReadMessage()
        if err != nil {
            log.Println(err)
            return
        }
        log.Println("Received message:", string(p))

        // 发送消息
        err = conn.WriteMessage(messageType, p)
        if err != nil {
            log.Println(err)
            return
        }
    }
}

func main() {
	log.Println("Listening...")
/*	http.HandleFunc("/listen", lsHandler)
	http.HandleFunc("/transmit", tsHandler)
	http.HandleFunc("/dw", rayHandler)
*/
  http.HandleFunc("/", defHandler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
