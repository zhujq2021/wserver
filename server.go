package main

import (
	"io"
	"log"
	"net"
	"net/http"

	"github.com/gorilla/websocket"
)

const port = "80"
const target = "127.0.0.1:22"
const v2proxy = "127.0.0.1:8080"

type client struct {
	listenChannel        chan bool // Channel that the client is listening on
	transmitChannel      chan bool // Channel that the client is writing to
	listener             io.Writer // The thing to write to
	listenerConnected    bool
	transmitter          io.Reader // The thing to listen from
	transmitterConnected bool
}

var upgrader = websocket.Upgrader{}
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
			_, err := io.Copy(connectedClients[clientId].listener, serverConn)
			if err != nil {
				log.Println("Down Conn Disconnect:", err)
			}

			wait <- true
		}()

		go func() {
			_, err := io.Copy(serverConn, connectedClients[clientId].transmitter)
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
	resolvedId := ""
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Error during connection upgradation of ls:", err)
		return
	}
	defer conn.Close()
	for {
		_, message, _ := conn.ReadMessage() //读取客户端发生的clientid
		line := string(message)
		if len(line) > 10 && (line[:10] == "Clientid: " || line[:10] == "clientid: ") {
			log.Println("Found clientid!")
			resolvedId = line[10:30]
			log.Println(resolvedId)
			break
		}
	}
	wait := make(chan bool)

	if _, ok := connectedClients[resolvedId]; !ok {
		connectedClients[resolvedId] = client{}
	}

	currentClient := connectedClients[resolvedId]

	messageType, _, err := conn.NextReader()
	if err != nil {
		log.Println(err)
		return
	}
	writer, err := conn.NextWriter(messageType)
	if err != nil {
		log.Println(err)
		return
	}

	currentClient.listener = writer
	currentClient.listenChannel = wait
	currentClient.listenerConnected = true

	connectedClients[resolvedId] = currentClient

	log.Println("Attempting to bind listener")

	go bindServer(resolvedId)

	<-wait

}

func tsHandler(w http.ResponseWriter, r *http.Request) {
	resolvedId := ""
	conn, err := upgrader.Upgrade(w, r, nil)
	//	reader := bufio.NewReader(conn)
	if err != nil {
		log.Print("Error during connection upgradation of ts:", err)
		return
	}
	defer conn.Close()
	for {
		_, message, _ := conn.ReadMessage()
		line := string(message) //读取客户端发生的clientid
		if len(line) > 10 && (line[:10] == "Clientid: " || line[:10] == "clientid: ") {
			log.Println("Found clientid!")
			resolvedId = line[10:30]
			log.Println(resolvedId)
			break
		}
	}

	wait := make(chan bool)

	if _, ok := connectedClients[resolvedId]; !ok {
		connectedClients[resolvedId] = client{}
	}

	currentClient := connectedClients[resolvedId]
	_, reader, err := conn.NextReader()
	if err != nil {
		log.Println(err)
		return
	}
	currentClient.transmitter = reader
	currentClient.transmitChannel = wait
	currentClient.transmitterConnected = true

	connectedClients[resolvedId] = currentClient

	log.Println("Attempting to bind transmission")

	go bindServer(resolvedId)

	<-wait

}

func defHandler(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
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

func rayHandler(w http.ResponseWriter, r *http.Request) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	server, err := net.Dial("tcp", v2proxy)
	if err != nil {
		log.Println("error to connect to v2ray:", err)
		return
	}
	server.Write([]byte("GET /dw HTTP/1.1\r\n\r\n"))
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
