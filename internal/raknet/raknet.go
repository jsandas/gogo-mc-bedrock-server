package raknet

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/sandertv/go-raknet"
)

type Pong struct {
	Edition         string
	ServerName      string // also called MOTD line 1
	ProtocolVersion int
	VersionName     string
	PlayerCount     int
	MaxPlayerCount  int
	ServerID        string
	LevelName       string // also called MOTD line 2
	GameMode        string
	GameModeInt     int
	IPv4Port        int
	IPv6Port        int
}

func GetPong(addr string) (Pong, error) {
	var msg Pong

	data, err := raknet.Ping(addr)
	if err != nil {
		return msg, fmt.Errorf("error pinging %s: %w", addr, err)
	}

	arr := bytes.Split(data, []byte(";"))

	msg = Pong{
		Edition:     string(arr[0]),
		ServerName:  string(arr[1]),
		VersionName: string(arr[3]),
		ServerID:    string(arr[6]),
		LevelName:   string(arr[7]),
		GameMode:    string(arr[8]),
	}

	msg.PlayerCount, _ = strconv.Atoi(string(arr[4]))
	msg.MaxPlayerCount, _ = strconv.Atoi(string(arr[5]))
	msg.GameModeInt, _ = strconv.Atoi(string(arr[9]))
	msg.IPv4Port, _ = strconv.Atoi(string(arr[10]))
	msg.IPv6Port, _ = strconv.Atoi(string(arr[11]))

	return msg, nil
}
