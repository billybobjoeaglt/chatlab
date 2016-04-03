package chat

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/billybobjoeaglt/chatlab/config"
	"github.com/billybobjoeaglt/chatlab/crypt"
	"github.com/billybobjoeaglt/chatlab/ui"
)

var outputChannel = make(chan chan string, 5)
var peers []Peer
var peersLock = &sync.Mutex{}
var messagesReceivedAlready = make(map[string]bool)
var messagesReceivedAlreadyLock = &sync.Mutex{}

type Peer struct {
	conn     net.Conn
	username string
}

func GetOutputChannel() chan chan string {
	return outputChannel
}

func CreateConnection(ip string) {
	go func() {
		conn, err := net.Dial("tcp", ip)
		if err == nil {
			handleConn(conn)
		} else {
			panic(err)
		}
	}()
}
func BroadcastMessage(message string) {
	encrypted, err := crypt.Encrypt(message, []string{"slaidan_lt", "leijurv"})
	if err != nil {
		panic(err)
	}
	broadcastEncryptedMessage(encrypted)
}
func broadcastEncryptedMessage(encrypted string) {
	tmpCopy := peers
	for i := range tmpCopy {
		fmt.Println("Sending to " + tmpCopy[i].username)
		tmpCopy[i].conn.Write([]byte(encrypted + "\n"))
	}
}
func onMessageReceived(message string, peerFrom Peer) {
	messagesReceivedAlreadyLock.Lock()
	_, found := messagesReceivedAlready[message]
	if found {
		fmt.Println("Lol wait. " + peerFrom.username + " sent us something we already has. Ignoring...")
		messagesReceivedAlreadyLock.Unlock()
		return
	}
	messagesReceivedAlready[message] = true
	messagesReceivedAlreadyLock.Unlock()
	messageChannel := make(chan string, 100)
	outputChannel <- messageChannel
	go func() {
		defer close(messageChannel)
		processMessage(message, messageChannel, peerFrom)
	}()
}
func processMessage(message string, messageChannel chan string, peerFrom Peer) {
	messageChannel <- "Relayed from "
	messageChannel <- peerFrom.username
	messageChannel <- ": "
	md, err := crypt.Decrypt(message)
	if err != nil {
		messageChannel <- "Unable to decrypt =("
		messageChannel <- err.Error()
		return
	}
	for k := range md.SignedBy.Entity.Identities {
		/*fmt.Println("Name: " + md.SignedBy.Entity.Identities[k].UserId.Name)
		fmt.Println("Email: " + md.SignedBy.Entity.Identities[k].UserId.Email)
		fmt.Println("Comment: " + md.SignedBy.Entity.Identities[k].UserId.Comment)
		fmt.Println("Creation Time: " + md.SignedBy.Entity.Identities[k].SelfSignature.CreationTime.Format(time.UnixDate) + "\n")
		*/

		messageChannel <- md.SignedBy.Entity.Identities[k].UserId.Name
		break
	}

	messageChannel <- ": "

	bytes, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		return
	}

	messageChannel <- string(bytes)
}

func handleConn(conn net.Conn) {
	fmt.Println("CONNECTION BABE. Sending our name")
	conn.Write([]byte(config.GetConfig().Username + "\n"))
	username, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return
	}
	username = strings.TrimSpace(username)
	fmt.Println("Received username: " + username)
	//here make sure that username is valid
	peer := Peer{conn: conn, username: username}
	peersLock.Lock()
	if peerWithName(peer.username) == -1 {
		peers = append(peers, peer)
		ui.AddUser(peer.username)
		peersLock.Unlock()
		go peerListen(peer)
	} else {
		peersLock.Unlock()
		peer.conn.Close()
		fmt.Println("Sadly we are already connected to " + peer.username + ". Disconnecting")
	}
}
func onConnClose(peer Peer) {
	//remove from list of peers, but idk how to do that in go =(
	fmt.Println("Disconnected from " + peer.username)
	peersLock.Lock()
	index := peerWithName(peer.username)
	if index == -1 {
		peersLock.Unlock()
		fmt.Println("lol what")
		return
	}
	peers = append(peers[:index], peers[index+1:]...)
	peersLock.Unlock()
}
func peerListen(peer Peer) {
	defer peer.conn.Close()
	defer onConnClose(peer)
	conn := peer.conn
	username := peer.username
	fmt.Println("Beginning to listen to " + username)
	for {
		message, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		message = strings.TrimSpace(message)
		onMessageReceived(message, peer)
	}
}
func peerWithName(name string) int {
	for i := 0; i < len(peers); i++ {
		if peers[i].username == name {
			return i
		}
	}
	return -1
}
func Listen(port int) {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		go handleConn(conn)
	}
}