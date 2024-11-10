package gol

import (
	"flag"
	"fmt"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	go startIo(p, ioChannels{})
	c.ioCommand <- ioInput

	c.ioFilename <- fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)

	nextWorld := make([][]byte, p.ImageHeight)
	for i := range nextWorld {
		nextWorld[i] = make([]byte, p.ImageWidth)
	}

	flippedCells := []util.Cell{}

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {

			pixelValue := <-c.ioInput
			if pixelValue == 255 {
				flipped := util.Cell{x, y}
				flippedCells = append(flippedCells, flipped)
			}
			nextWorld[y][x] = pixelValue
		}
	}

	currentWorld := make(chan [][]uint8)
	currentTurn := make(chan int)
	responseChannel := make(chan stubs.FinalResponse)
	done := make(chan bool)

	//requestChan := make(chan struct{})
	//pauseSignal := make(chan struct{})
	//playSignal := make(chan struct{})
	//quitSignal := make(chan struct{})
	//saveSignal := make(chan struct{})
	// Channel for request signals

	//var ticker *time.Ticker
	//ticker = time.NewTicker(2 * time.Second)
	//go background(requestChan, p, c, currentWorld, currentTurn, done, ticker)
	//go keyListener(c, pauseSignal, playSignal, quitSignal, saveSignal)

	turn := 0
	//c.events <- CellsFlipped{turn, flippedCells}
	c.events <- StateChange{turn, Executing}

	c.ioCommand <- ioOutput
	//filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Threads)

	// TODO: Execute all turns of the Game of Life.
	server := flag.String("server", "127.0.0.1:8030", "IP:port for the AWS master") //change the ip to amazon
	flag.Parse()
	client, _ := rpc.Dial("tcp", *server)
	defer client.Close()
	makeCall(client, nextWorld, p.Turns, p.Threads, responseChannel)
	nuke := <-responseChannel
	nukeAlive := nuke.AliveCells
	nukeCompletedTurns := nuke.TurnsCompleted
	//nukeFinalWorld := nuke.FinalWorld

	// TODO: Report the final state using FinalTurnCompleteEvent.
	close(done)
	close(currentTurn)
	close(currentWorld)
	c.events <- FinalTurnComplete{CompletedTurns: nukeCompletedTurns, Alive: nukeAlive}
	//c.ioFilename <- fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, turn)

	//for y := 0; y < p.ImageHeight; y++ {
	//for x := 0; x < p.ImageWidth; x++ {

	//pixelValue := nextWorld[y][x]
	//c.ioOutput <- pixelValue
	//}
	//}

	//c.ioCommand <- ioCheckIdle
	//<-c.ioIdle
	//c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filename}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	// Make sure that the Io has finished any output before exiting.

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)

}

func makeCall(client *rpc.Client, worldProcess [][]byte, turns int, threads int, responseChannel chan stubs.FinalResponse) {
	request := stubs.InitialRequest{NextWorld: worldProcess, Turns: turns, ThreadCount: threads}
	response := new(stubs.FinalResponse)
	client.Call(stubs.StartMaster, request, response)
	//fmt.Println("Responded: " + response.Message)
	responseChannel <- *response
}

func keyListener(c distributorChannels, pauseSignal chan struct{}, playSignal chan struct{}, quitSignal chan struct{}, saveSignal chan struct{}) {
	playState := "play"
	for {
		select {
		case keyPressed := <-c.keyPresses:
			if keyPressed == 'p' && playState == "play" {
				pauseSignal <- struct{}{}
				playState = "pause"
			} else if keyPressed == 'p' && playState == "pause" {
				playSignal <- struct{}{}
				playState = "play"
			} else if keyPressed == 'q' {
				quitSignal <- struct{}{}
			} else if keyPressed == 's' {
				saveSignal <- struct{}{}
			}
		}
	}
}

func background(requestChan chan struct{}, p Params, c distributorChannels, currentWorld chan [][]byte, currentTurn chan int, done chan bool, ticker *time.Ticker) {
	time.Sleep(2 * time.Second)
	for {
		select {
		case <-done:
			ticker.Stop()
			return
		case <-ticker.C:
			requestChan <- struct{}{}
			currentTurn2 := <-currentTurn
			currentWorld2 := <-currentWorld
			fmt.Println(currentWorld2)
			c.events <- AliveCellsCount{CompletedTurns: currentTurn2, CellsCount: 0}
		}
	}
}
