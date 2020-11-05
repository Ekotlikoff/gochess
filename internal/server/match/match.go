package matchserver

import (
	"errors"
	"gochess/internal/model"
	"time"
)

type Match struct {
	black         *Player
	white         *Player
	game          *model.Game
	gameOver      chan struct{}
	maxTimeMs     int64
	requestedDraw *Player
}

type MatchGenerator func(black *Player, white *Player) Match

func newMatch(black *Player, white *Player, maxTime int64) Match {
	black.color = model.Black
	white.color = model.White
	if black.name == white.name {
		black.name = black.name + "_black"
		white.name = white.name + "_white"
	}
	black.elapsedMs = 0
	white.elapsedMs = 0
	game := model.NewGame()
	return Match{black, white, &game, make(chan struct{}), maxTime, nil}
}

func DefaultMatchGenerator(black *Player, white *Player) Match {
	return newMatch(black, white, 1200000)
}

func (match *Match) play() {
	go match.handleAsyncRequests()
	for !match.game.GameOver() {
		match.handleTurn()
	}
}

func (match *Match) handleTurn() {
	player := match.black
	opponent := match.white
	if match.game.Turn() != model.Black {
		player = match.white
		opponent = match.black
	}
	turnStart := time.Now()
	timeRemaining := match.maxTimeMs - player.elapsedMs
	timer := time.AfterFunc(time.Duration(timeRemaining)*time.Millisecond,
		match.handleTimeout(opponent))
	defer timer.Stop()
	request := RequestSync{}
	select {
	case request = <-player.requestChanSync:
	case <-match.gameOver:
		return
	}
	err := errors.New("")
	for err != nil {
		err = match.game.Move(request.position, request.move)
		if err != nil {
			select {
			case player.responseChanSync <- ResponseSync{moveSuccess: false}:
			case <-match.gameOver:
				return
			}
			select {
			case request = <-player.requestChanSync:
			case <-match.gameOver:
				return
			}
		}
	}
	player.responseChanSync <- ResponseSync{moveSuccess: true}
	if match.game.GameOver() {
		result := match.game.Result()
		winner := match.black
		if result.Winner == model.White {
			winner = match.white
		}
		match.handleGameOver(result.Draw, false, false, winner)
	}
	player.elapsedMs += time.Now().Sub(turnStart).Milliseconds()
}

func (match *Match) handleTimeout(opponent *Player) func() {
	return func() {
		match.handleGameOver(false, false, true, opponent)
	}
}

func (match *Match) handleAsyncRequests() {
	for !match.game.GameOver() {
		opponent := match.white
		player := match.black
		request := RequestAsync{}
		select {
		case request = <-match.black.requestChanAsync:
		case request = <-match.white.requestChanAsync:
			opponent = match.black
			player = match.white
		case <-match.gameOver:
			return
		}
		if request.resign {
			match.handleGameOver(false, true, false, opponent)
			return
		} else if request.requestToDraw {
			if match.requestedDraw == opponent {
				match.handleGameOver(true, false, false, opponent)
			} else if match.requestedDraw == player {
				// Consider the second requestToDraw a toggle.
				match.requestedDraw = nil
			} else {
				match.requestedDraw = player
				go func() {
					select {
					case opponent.responseChanAsync <- ResponseAsync{
						false, true, false, false, false, "",
					}:
					case <-time.After(5 * time.Second):
					}
				}()
			}
		}
	}
}

func (match *Match) handleGameOver(
	draw, resignation, timeout bool, winner *Player,
) {
	match.game.SetGameResult(winner.color, draw)
	winnerName := winner.name
	if draw {
		winnerName = ""
	}
	response := ResponseAsync{gameOver: true, draw: draw,
		resignation: resignation, timeout: timeout, winner: winnerName}
	for _, player := range [2]*Player{match.white, match.black} {
		go func() {
			select {
			case player.responseChanAsync <- response:
			case <-time.After(5 * time.Second):
			}
		}()
	}
	close(match.gameOver)
}