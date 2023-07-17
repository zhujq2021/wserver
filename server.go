package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

const port = "80"
const target = "127.0.0.1:22"
const v2proxy = "127.0.0.1:8080"

func sshHandler(w *websocket.Conn) {

	serverConn, err := net.Dial("tcp", target)
	if err != nil {
		log.Println("Failed to connect to remote server :/", err)
		return
	}
	log.Println("success to dial: " + target)

	defer serverConn.Close()
	_, err = io.Copy(serverConn, w)
	if err != nil {
		log.Println("UP Conn Disconnect:", err)
		return
	}

	go func() {
		_, err := io.Copy(w, serverConn)
		if err != nil {
			log.Println("Down Conn Disconnect:", err)
			return
		}
	}()

}

func defHandler(w *websocket.Conn) {
	var err error
	for {
		var reply string
		if err = websocket.Message.Receive(w, &reply); err != nil {
			log.Println("接受消息失败", err)
			break
		}
		msg := time.Now().String() + reply
		//log.Println(msg)
		if err = websocket.Message.Send(w, msg); err != nil {
			log.Println("发送消息失败")
			break
		}
	}
}

func rayHandler(w http.ResponseWriter, r *http.Request) {
	str := "GET /dw HTTP/1.1\r\n"
	str += ("Host: " + r.Host + "\r\n")
	for k, v := range r.Header {
		str += (k + ": " + strings.Join(v, ",") + "\r\n")
	}
	str += "\r\n"
	log.Println(str)

	//log.Println("Getting ray access request...")
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Println("Hijacker error")
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Println("Hijacker Conn error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//log.Println("hj.hijacker is ok")
	defer clientConn.Close()

	server, err := net.Dial("tcp", v2proxy)
	if err != nil {
		log.Println("error to connect to v2ray:", err)
		return
	}

	server.Write([]byte(str))
	go io.Copy(server, clientConn)
	io.Copy(clientConn, server)

}

func main() {
	log.Println("Listening...")
	http.Handle("/ssh", websocket.Handler(sshHandler))
	http.HandleFunc("/dw", rayHandler)
	http.Handle("/", websocket.Handler(defHandler))
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
