package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"sync"
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
	responseChannel := make(chan stubs.FinalResponse)
	done := make(chan bool)

	fmt.Println("ive made it here")

	//requestChan := make(chan struct{})
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
	client, _ := rpc.Dial("tcp", "ec2-3-233-250-228.compute-1.amazonaws.com:8030")
	defer client.Close()
	var ticker *time.Ticker
	ticker = time.NewTicker(2 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go background(client, done, ticker, c, &wg)
	wg.Add(1)
	values2 := makeCall(client, nextWorld, p.Turns, p.Threads, responseChannel, &wg)
	wg.Wait()
	//nuke := values.World
	nukeAlive := values2.AliveCells
	nukeCompletedTurns := values2.TurnCompleted
	//nukeFinalWorld := nuke.FinalWorld

	// TODO: Report the final state using FinalTurnCompleteEvent.
	//close(done)

	c.events <- FinalTurnComplete{CompletedTurns: nukeCompletedTurns, Alive: nukeAlive}
	close(done)

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

	//c.ioCommand <- ioCheckIdle
	//<-c.ioIdle
	fmt.Println("im here now4", p.Turns, p.Threads)

	// Make sure that the Io has finished any output before exiting.

	c.events <- StateChange{turn, Quitting}
	fmt.Println("im here now5", p.Turns, p.Threads)

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)

}

type Value struct {
	World         [][]byte
	TurnCompleted int
	AliveCells    []util.Cell
}

func makeCall(client *rpc.Client, worldProcess [][]byte, turns int, threads int, responseChannel chan stubs.FinalResponse, wg *sync.WaitGroup) Value {
	defer wg.Done()
	request := stubs.InitialRequest{NextWorld: worldProcess, Turns: turns, ThreadCount: threads}
	if client == nil {
		log.Fatal("RPC client is nil. Could not connect to the server.")
	}

	response := new(stubs.FinalResponse)
	fmt.Println("im here now3")

	err := client.Call(stubs.StartMaster, request, response)
	if err != nil {
		log.Fatalf("Error during RPC call2: %v", err)
	}

	fmt.Printf("passingBack from server.\n")

	return Value{AliveCells: response.AliveCells, World: response.FinalWorld, TurnCompleted: response.TurnsCompleted}
}

func keyListener(c distributorChannels) {
	playState := "play"
	for {
		select {
		case keyPressed := <-c.keyPresses:
			if keyPressed == 'p' && playState == "play" {
				playState = "pause"
			} else if keyPressed == 'p' && playState == "pause" {
				playState = "play"
			} else if keyPressed == 'q' {
			} else if keyPressed == 's' {
			}
		}
	}
}

func background(client *rpc.Client, done chan bool, ticker *time.Ticker, c distributorChannels, wg *sync.WaitGroup) {
	time.Sleep(2 * time.Second)
	defer wg.Done()
	for {
		select {
		case <-done:
			ticker.Stop()
			return
		case <-ticker.C:
			request := stubs.AliveRequest{TimeToRequest: true}
			if client == nil {
				log.Fatal("RPC client is nil. Could not connect to the server.")
			}

			response := new(stubs.AliveResponse)

			err := client.Call(stubs.RunTicker, request, response)
			if err != nil {
				log.Fatalf("Error during RPC call1: %v", err)
			}

			fmt.Printf("passingBack from server.\n")

			c.events <- AliveCellsCount{CompletedTurns: response.CurrentTurns, CellsCount: response.AliveCellCount}

		}
	}
}
