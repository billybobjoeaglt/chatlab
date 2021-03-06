package main

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/billybobjoeaglt/chatlab/chat"
	"github.com/billybobjoeaglt/chatlab/common"
	"github.com/billybobjoeaglt/chatlab/config"
	"github.com/billybobjoeaglt/chatlab/logger"
	"github.com/billybobjoeaglt/chatlab/ui"
	"github.com/billybobjoeaglt/chatlab/ui/startup"
	"github.com/codegangsta/cli"
)

var (
	Version   string
	BuildTime string
)

func main() {
	// Defining cli params for app
	app := cli.NewApp()
	app.Name = "ChatLab"
	app.Usage = "A P2P encrypted chat client using keybase.io"
	app.Authors = []cli.Author{
		cli.Author{
			Name: "Aidan Lloyd-Tucker",
		},
		cli.Author{
			Name: "Leif",
		},
	}
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "port, p",
			Value: common.DefaultPort,
			Usage: "set port of client",
		},
		cli.BoolFlag{
			Name:  "oldcli, o",
			Usage: "Disables CLI",
		},
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "Enables verbosity",
		},
		cli.BoolFlag{
			Name:  "relay, r",
			Usage: "Enables relay mode",
		},
	}
	app.UsageText = "chatlab [arguments...]"
	app.Version = Version
	num, err := strconv.ParseInt(BuildTime, 10, 64)
	if err == nil {
		app.Compiled = time.Unix(num, 0)
	}
	app.Action = runApp
	app.Run(os.Args)

}

// This gets called when the app is run
func runApp(c *cli.Context) {
	var err error

	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	common.ProgramDir = filepath.Join(usr.HomeDir, ".chatlab")

	_, err = os.Stat(common.ProgramDir)
	if os.IsNotExist(err) {
		os.MkdirAll(common.ProgramDir, 0777)
	}

	// Loads Config
	err = config.LoadConfig()
	if err != nil {
		panic(err)
	}

	err = chat.LoadPeers()
	if err != nil {
		panic(err)
	}

	// Starts a process to getting new messages and sending them to the ui
	go uiPrint(chat.GetMessageChannel())

	// Starts a process listening on the given port
	go chat.Listen(c.Int("port"))

	// Gets IP
	var ip string = "UNKNOWN"

	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				ip = net.JoinHostPort(ipnet.IP.String(), strconv.Itoa(c.Int("port")))
			}
		}
	}

	fmt.Println("Broadcasting on: " + ip)

	chat.SelfNode.Username = config.GetConfig().Username
	chat.SelfNode.Port = strconv.Itoa(c.Int("port"))

	// Chooses which UI to use
	if c.Bool("relay") {
		chat.SelfNode.IsRelay = true
		const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
		var bytes = make([]byte, 15)
		rand.Read(bytes)
		for i, b := range bytes {
			bytes[i] = alphanum[b%byte(len(alphanum))]
		}
		chat.SelfNode.Username = string(bytes)
		ui.NewRelayConsole()
	} else {
		startup.RunStartup()
		if c.Bool("oldcli") {
			err = ui.NewCLI()
			if err != nil {
				panic(err)
			}
		} else {
			err = ui.NewCUI()
			if err != nil {
				panic(err)
			}
		}
	}

	// Sets verbosity
	logger.IsVerbose = c.Bool("verbose")

	// Gives the ui package functions to connect with the chat package
	ui.SetSendMessage(func(msg common.Message) {
		go chat.BroadcastMessage(msg)
	})
	ui.SetCreateConn(func(ip string) {
		go chat.CreateConnection(ip, false)
	})

	// Safe Exit
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		common.Done <- true
	}()
	<-common.Done
	err = config.SaveConfig()
	if err != nil {
		panic(err)
	}
	err = chat.SavePeers()
	if err != nil {
		panic(err)
	}
	ui.Quit()
	fmt.Println("Safe Exited")
}

// Gets new messages from a channel and gives them to the ui
func uiPrint(msgChan <-chan common.Message) {
	for {
		msg, ok := <-msgChan
		if ok {
			ui.AddMessage(msg)
		}
	}
}
