package main

import (
	"fmt"
	"gochess/internal/model"
	"strconv"
	"syscall/js"
)

var game model.Game
var playerColor model.Color
var elDragging js.Value
var pieceDragging *model.Piece
var originalTransform js.Value
var positionOriginal model.Position
var isMouseDown bool
var document = js.Global().Get("document")
var board = document.Call("getElementById", "board-layout-chessboard")

func main() {
	done := make(chan struct{}, 0)
	initStyle()
	game = model.NewGame()
	playerColor = model.White
	// TODO make http calls to interact with server
	js.Global().Set("add", js.FuncOf(add))
	js.Global().Set("subtract", js.FuncOf(subtract))
	document.Call("addEventListener", "mousemove", js.FuncOf(mouseMove), false)
	document.Call("addEventListener", "mouseup", js.FuncOf(mouseUp), false)
	board.Call("addEventListener", "contextmenu", js.FuncOf(preventDefault), false)
	initBoard(model.White)
	<-done
}

func initStyle() {
	styleEl := document.Call("createElement", "style")
	document.Get("head").Call("appendChild", styleEl)
	styleSheet := styleEl.Get("sheet")
	for x := 1; x < 9; x++ {
		for y := 1; y < 9; y++ {
			selector := ".square-" + strconv.Itoa(x*10+y)
			transform := fmt.Sprintf(
				"{transform: translate(%d%%,%d%%);}",
				x*100-100, 700-((y-1)*100),
			)
			styleSheet.Call("insertRule", selector+transform)
		}
	}
}

func resetBoard() {
	elements := document.Call("getElementsByClassName", "piece")
	for i := 0; i < elements.Length(); i++ {
		elements.Index(i).Call("remove")
	}
}

func initBoard(playerColor model.Color) {
	for _, file := range game.Board() {
		for _, piece := range file {
			if piece != nil {
				div := document.Call("createElement", "div")
				div.Get("classList").Call("add", "piece")
				div.Get("classList").Call("add", piece.ClientString())
				div.Get("classList").Call(
					"add", getPositionClass(piece.Position(), playerColor))
				board.Call("appendChild", div)
				div.Call("addEventListener", "mousedown",
					js.FuncOf(mouseDown), false)
			}
		}
	}
}

func preventDefault(this js.Value, i []js.Value) interface{} {
	if len(i) > 0 {
		i[0].Call("preventDefault")
	}
	return 0
}

func getPositionClass(position model.Position, playerColor model.Color) string {
	class := "square-"
	if playerColor == model.Black {
		class += strconv.Itoa(int(8 - position.File))
		class += strconv.Itoa(int(8 - position.Rank))
	} else {
		class += strconv.Itoa(int(position.File + 1))
		class += strconv.Itoa(int(position.Rank + 1))
	}
	return class
}

func mouseDown(this js.Value, i []js.Value) interface{} {
	if len(i) > 0 && !isMouseDown {
		isMouseDown = true
		i[0].Call("preventDefault")
		elDragging = this
		_, _, _, _, gridX, gridY := getEventMousePosition(i[0])
		positionOriginal = model.Position{uint8(gridX), uint8(7 - gridY)}
		pieceDragging = game.Board()[gridX][7-gridY]
		originalTransform = elDragging.Get("style").Get("transform")
		elDragging.Get("classList").Call("add", "dragging")
	}
	return 0
}

func mouseMove(this js.Value, i []js.Value) interface{} {
	i[0].Call("preventDefault")
	if isMouseDown {
		x, y, squareWidth, squareHeight, _, _ := getEventMousePosition(i[0])
		pieceWidth := elDragging.Get("clientWidth").Float()
		pieceHeight := elDragging.Get("clientHeight").Float()
		percentX := 100 * (float64(x) - pieceWidth/2) / float64(squareWidth)
		percentY := 100 * (float64(y) - pieceHeight/2) / float64(squareHeight)
		elDragging.Get("style").Set("transform",
			"translate("+fmt.Sprintf("%f%%,%f%%)", percentX, percentY))
	}
	return 0
}

func mouseUp(this js.Value, i []js.Value) interface{} {
	if isMouseDown && len(i) > 0 {
		i[0].Call("preventDefault")
		elDragging.Get("style").Set("transform", originalTransform)
		elDragging.Get("style").Set("transform", originalTransform)
		elDragging.Get("classList").Call("remove", "dragging")
		elDragging.Get("classList").Call("remove", "dragging")
		originalPositionClass := getPositionClass(positionOriginal, playerColor)
		_, _, _, _, gridX, gridY := getEventMousePosition(i[0])
		positionDragging := model.Position{uint8(gridX), uint8(7 - gridY)}
		move := model.Move{
			int8(positionDragging.File) - int8(positionOriginal.File),
			int8(positionDragging.Rank) - int8(positionOriginal.Rank),
		}
		err := game.Move(positionOriginal, move)
		fmt.Println(err)
		if err == nil {
			newPositionClass := getPositionClass(positionDragging, playerColor)
			elements := document.Call("getElementsByClassName", newPositionClass)
			elementsLength := elements.Length()
			for i := 0; i < elementsLength; i++ {
				elements.Index(i).Call("remove")
			}
			elDragging.Get("classList").Call("remove", originalPositionClass)
			elDragging.Get("classList").Call("add", newPositionClass)
			handleCastle(pieceDragging, move)
			handleEnPassant(pieceDragging, move, elementsLength == 0)
		}
		elDragging = js.Undefined()
		isMouseDown = false
	}
	return 0
}

func getEventMousePosition(event js.Value) (int, int, int, int, int, int) {
	rect := board.Call("getBoundingClientRect")
	width := rect.Get("right").Int() - rect.Get("left").Int()
	height := rect.Get("bottom").Int() - rect.Get("top").Int()
	squareWidth := width / 8
	squareHeight := height / 8
	x := event.Get("clientX").Int() - rect.Get("left").Int()
	gridX := x / squareWidth
	if x > width || gridX > 7 {
		x = width
		gridX = 7
	} else if x < 0 || gridX < 0 {
		x = 0
		gridX = 0
	}
	y := event.Get("clientY").Int() - rect.Get("top").Int()
	gridY := y / squareHeight
	if y > height || gridY > 7 {
		y = height
		gridY = 7
	} else if y < 0 || gridY < 0 {
		y = 0
		gridY = 0
	}
	return x, y, squareWidth, squareHeight, gridX, gridY
}

func handleEnPassant(pawn *model.Piece, move model.Move, targetEmpty bool) {
	if pieceDragging.PieceType() == model.Pawn && move.X != 0 && targetEmpty {
		capturedY := pawn.Rank() + 1
		if move.Y > 0 {
			capturedY = pawn.Rank() - 1
		}
		capturedPosition := model.Position{pawn.File(), capturedY}
		capturedPosClass := getPositionClass(capturedPosition, playerColor)
		elements := document.Call("getElementsByClassName", capturedPosClass)
		elementsLength := elements.Length()
		for i := 0; i < elementsLength; i++ {
			elements.Index(i).Call("remove")
		}
	}
}

func handleCastle(king *model.Piece, move model.Move) {
	if pieceDragging.PieceType() == model.King &&
		(move.X < -1 || move.X > 1) {
		var rookPosition model.Position
		var rookPosClass string
		var rookNewPosClass string
		if king.File() == 2 {
			rookPosition = model.Position{0, king.Rank()}
			rookPosClass = getPositionClass(rookPosition, playerColor)
			newRookPosition := model.Position{3, king.Rank()}
			rookNewPosClass = getPositionClass(newRookPosition, playerColor)
		} else if king.File() == 6 {
			rookPosition = model.Position{7, king.Rank()}
			rookPosClass = getPositionClass(rookPosition, playerColor)
			newRookPosition := model.Position{5, king.Rank()}
			rookNewPosClass = getPositionClass(newRookPosition, playerColor)
		} else {
			panic("King not in valid castled position")
		}
		elements := document.Call("getElementsByClassName", rookPosClass)
		elementsLength := elements.Length()
		for i := 0; i < elementsLength; i++ {
			elements.Index(i).Get("classList").Call("add", rookNewPosClass)
			elements.Index(i).Get("classList").Call("remove", rookPosClass)
		}
	}
}

func add(this js.Value, i []js.Value) interface{} {
	value1 := document.Call("getElementById", i[0].String()).Get("value").String()
	value2 := document.Call("getElementById", i[1].String()).Get("value").String()
	int1, _ := strconv.Atoi(value1)
	int2, _ := strconv.Atoi(value2)
	document.Call("getElementById", i[2].String()).Set("value", int1+int2)
	return 0
}

func subtract(this js.Value, i []js.Value) interface{} {
	value1 := document.Call("getElementById", i[0].String()).Get("value").String()
	value2 := document.Call("getElementById", i[1].String()).Get("value").String()
	int1, _ := strconv.Atoi(value1)
	int2, _ := strconv.Atoi(value2)
	document.Call("getElementById", i[2].String()).Set("value", int1-int2)
	return 0
}