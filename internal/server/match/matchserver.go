package matchserver

import (
	"gochess/internal/model"
	"sync"
)

type Player struct {
	name               string
	color              model.Color
	elapsedMs          int64
	requestChanSync    chan RequestSync
	responseChanSync   chan ResponseSync
	requestChanAsync   chan RequestAsync
	responseChanAsync  chan ResponseAsync
	opponentPlayedMove chan model.MoveRequest
	matchStart         chan struct{}
	matchedOpponent    string
}

func NewPlayer(name string) Player {
	return Player{
		name, model.Black, int64(0),
		make(chan RequestSync, 1), make(chan ResponseSync, 10),
		make(chan RequestAsync, 1), make(chan ResponseAsync, 1),
		make(chan model.MoveRequest, 10), make(chan struct{}), "",
	}
}

func (player *Player) Name() string {
	return player.name
}

func (player *Player) MatchedOpponentName() string {
	return player.matchedOpponent
}

func (player *Player) Color() model.Color {
	return player.color
}

func (player *Player) WaitForMatchStart() {
	<-player.matchStart
}

func (player *Player) MakeMove(pieceMove model.MoveRequest) bool {
	player.requestChanSync <- RequestSync{pieceMove.Position, pieceMove.Move}
	response := <-player.responseChanSync
	return response.moveSuccess
}

func (player *Player) GetSyncUpdate() model.MoveRequest {
	return <-player.opponentPlayedMove
}

func (player *Player) RequestAsync(requestAsync RequestAsync) {
	player.requestChanAsync <- requestAsync
}

func (player *Player) GetAsyncUpdate() ResponseAsync {
	return <-player.responseChanAsync
}

func (player *Player) Stop() {
	close(player.requestChanSync)
	close(player.responseChanSync)
	close(player.requestChanAsync)
	close(player.responseChanAsync)
	close(player.matchStart)
}

type RequestSync struct {
	position model.Position
	move     model.Move
}

type ResponseSync struct {
	moveSuccess bool
}

type RequestAsync struct {
	RequestToDraw, Resign bool
}

type ResponseAsync struct {
	GameOver, RequestToDraw, Draw, Resignation, Timeout bool
	Winner                                              string
}

type MatchingServer struct {
	liveMatches  []*Match
	mutex        *sync.Mutex
	players      chan *Player
	pendingMatch *sync.Mutex
}

func NewMatchingServer() MatchingServer {
	return MatchingServer{
		mutex: &sync.Mutex{}, players: make(chan *Player),
		pendingMatch: &sync.Mutex{},
	}
}

func (matchingServer *MatchingServer) LiveMatches() []*Match {
	liveMatches := []*Match{}
	matchingServer.mutex.Lock()
	liveMatches = matchingServer.liveMatches
	matchingServer.mutex.Unlock()
	return liveMatches
}

func (matchingServer *MatchingServer) matchAndPlay(
	matchGenerator MatchGenerator, playServerID int,
) {
	var player1, player2 *Player
	// Lock until a full match is found and started, thus avoiding unmatched
	// players stranded across goroutines.
	matchingServer.pendingMatch.Lock()
	for player := range matchingServer.players {
		if player1 == nil {
			player1 = player
		} else if player2 == nil {
			player2 = player
			player1.matchedOpponent = player2.name
			player2.matchedOpponent = player1.name
			match := matchGenerator(player1, player2)
			matchingServer.mutex.Lock()
			matchingServer.liveMatches = append(matchingServer.liveMatches, &match)
			matchingServer.mutex.Unlock()
			matchingServer.pendingMatch.Unlock()
			close(player1.matchStart)
			close(player2.matchStart)
			(&match).play()
			matchingServer.removeMatch(&match)
			player1, player2 = nil, nil
			matchingServer.pendingMatch.Lock()
		}
	}
}

func (matchingServer *MatchingServer) StartMatchServers(
	maxConcurrentGames int, quit chan bool,
) {
	matchingServer.StartCustomMatchServers(
		maxConcurrentGames, DefaultMatchGenerator, quit,
	)
}

func (matchingServer *MatchingServer) StartCustomMatchServers(
	maxConcurrentGames int, matchGenerator MatchGenerator, quit chan bool,
) {
	// Start handlers
	for i := 0; i < maxConcurrentGames; i++ {
		go matchingServer.matchAndPlay(matchGenerator, i)
	}
	<-quit // Wait to be told to exit.
}

func (matchingServer *MatchingServer) removeMatch(matchToRemove *Match) {
	liveMatches := matchingServer.liveMatches
	matchingServer.mutex.Lock()
	defer matchingServer.mutex.Unlock()
	for i, match := range liveMatches {
		if match == matchToRemove {
			if len(liveMatches) == 1 {
				matchingServer.liveMatches = nil
			} else {
				liveMatches[i] = liveMatches[len(liveMatches)-1]
				liveMatches = liveMatches[:len(liveMatches)-1]
				return
			}
		}
	}
}

func (matchingServer *MatchingServer) MatchPlayer(player *Player) {
	matchingServer.players <- player
}
