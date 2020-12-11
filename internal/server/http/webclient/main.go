// +build webclient

package main

import (
	"github.com/Ekotlikoff/gochess/internal/model"
	"net/http"
	"net/http/cookiejar"
	"syscall/js"
)

var server string

func main() {
	done := make(chan struct{}, 0)
	game := model.NewGame()
	jar, _ := cookiejar.New(&cookiejar.Options{})
	client := &http.Client{Jar: jar}
	clientModel := ClientModel{
		game: &game, playerColor: model.White,
		document: js.Global().Get("document"),
		board: js.Global().Get("document").Call(
			"getElementById", "board-layout-chessboard"),
		matchingServerURI: server,
		client:            client,
	}
	clientModel.initController(quiet)
	clientModel.initStyle()
	clientModel.viewInitBoard(clientModel.playerColor)
	<-done
}
