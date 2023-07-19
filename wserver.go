package main

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/zhujq/websocket"
)

const port = "80"
const target = "127.0.0.1:22"
const v2proxy = "127.0.0.1:8080"

type client struct {
	listenChannel        chan bool // Channel that the client is listening on
	transmitChannel      chan bool // Channel that the client is writing to
	listener             net.Conn  // The thing to write to
	listenerConnected    bool
	transmitter          net.Conn // The thing to listen from
	transmitterConnected bool
}

var connectedClients map[string]client = make(map[string]client)

func bindServer(clientId string) {
	if connectedClients[clientId].listenerConnected && connectedClients[clientId].transmitterConnected {
		log.Println("Two-way connection to client established!")
		log.Println("Client <=|F|=> Proxy <-...-> VPN")

		defer func() {
			connectedClients[clientId].listenChannel <- true
			connectedClients[clientId].transmitChannel <- true
			delete(connectedClients, clientId)
		}()

		serverConn, err := net.Dial("tcp", target)
		if err != nil {
			log.Println("Failed to connect to remote server :/", err)
		}
		log.Println("success to dial" + target)

		defer serverConn.Close()

		wait := make(chan bool)

		go func() {
			_, err = io.Copy(connectedClients[clientId].listener, serverConn)
			if err != nil {
				log.Println("Down Conn Disconnect:", err)
			}

			wait <- true
		}()

		go func() {
			_, err = io.Copy(serverConn, connectedClients[clientId].transmitter)
			if err != nil {
				log.Println("Up Conn Disconnect:", err)
			}

			wait <- true
		}()

		log.Println("Full connection established!")
		log.Println("Client <=|F|=> Proxy <---> VPN")

		<-wait
		log.Println("Connection closed")
	}
}

func lsHandler(w http.ResponseWriter, r *http.Request) {
	ao := &websocket.AcceptOptions{InsecureSkipVerify: true}
	conn, err := websocket.Accept(w, r, ao)
	if err != nil {
		log.Println("webscoket accept err：", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "inner error！")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resolvedId := ""

	_, msg, err := conn.Read(ctx)
	if err != nil {
		log.Println("webscoket read clientid err：", err)
		return
	}
	line := string(msg[:])
	if len(line) > 10 && (line[:10] == "Clientid: " || line[:10] == "clientid: ") {
		resolvedId = line[10:]
		log.Println("Found clientid:" + resolvedId)
	} else {
		log.Println("webscoket read clientid err：", err)
		return
	}

	wait := make(chan bool)

	if _, ok := connectedClients[resolvedId]; !ok {
		connectedClients[resolvedId] = client{}
	}

	currentClient := connectedClients[resolvedId]

	currentClient.listener = websocket.NetConn(ctx, conn, websocket.MessageText)
	currentClient.listenChannel = wait
	currentClient.listenerConnected = true
	connectedClients[resolvedId] = currentClient

	log.Println("Attempting to bind listener")

	go bindServer(resolvedId)

	<-wait

}

func tsHandler(w http.ResponseWriter, r *http.Request) {
	ao := &websocket.AcceptOptions{InsecureSkipVerify: true}
	conn, err := websocket.Accept(w, r, ao)
	if err != nil {
		log.Println("webscoket accept err：", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "inner error！")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resolvedId := ""
	_, msg, err := conn.Read(ctx)
	if err != nil {
		log.Println("webscoket read clientid err：", err)
		return
	}
	line := string(msg[:])

	if len(line) > 10 && (line[:10] == "Clientid: " || line[:10] == "clientid: ") {
		resolvedId = line[10:]
		log.Println("Found clientid:" + resolvedId)
	} else {
		log.Println("webscoket read clientid err：", err)
		return
	}

	wait := make(chan bool)

	if _, ok := connectedClients[resolvedId]; !ok {
		connectedClients[resolvedId] = client{}
	}

	currentClient := connectedClients[resolvedId]

	currentClient.transmitter = websocket.NetConn(ctx, conn, websocket.MessageText)
	currentClient.transmitChannel = wait
	currentClient.transmitterConnected = true

	connectedClients[resolvedId] = currentClient

	log.Println("Attempting to bind transmission")

	go bindServer(resolvedId)

	<-wait

}

func defHandler(w http.ResponseWriter, r *http.Request) {
	ao := &websocket.AcceptOptions{InsecureSkipVerify: true}
	conn, err := websocket.Accept(w, r, ao)
	if err != nil {
		log.Println("webscoket accept err：", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "inner error！")
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	_, reader, err := conn.Reader(ctx)
	if err != nil {
		log.Println(":Error to get reader:", err)
		return
	}

	writer, err := conn.Writer(ctx, websocket.MessageBinary)
	if err != nil {
		log.Println(":Error to get writer:", err)
		return
	}
	io.Copy(writer, reader)
	conn.Close(websocket.StatusNormalClosure, "")
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
	http.HandleFunc("/listen", lsHandler)
	http.HandleFunc("/transmit", tsHandler)
	http.HandleFunc("/dw", rayHandler)
	http.HandleFunc("/", defHandler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
