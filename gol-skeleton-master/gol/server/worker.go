package main

import (
	"errors"
	"flag"
	"log"
	"net"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GameOfLifeOperations struct{}

func (g *GameOfLifeOperations) ProcessGameOfLife(req stubs.Request, res *stubs.Response) (err error) {
	if req.NextWorld == nil {
		err = errors.New("no final board recieved")
		return
	}

	nextWorld := req.NextWorld
	turns := req.Turns
	workerId := req.WorkerNumber
	threadCount := req.ThreadCount

	for localTurn := 0; localTurn < turns; localTurn++ {
		nextWorld = calculateNextState(workerId, nextWorld, threadCount)
	}

	// Set the FinalWorld and Turns in the response
	res.FinalWorld = nextWorld
	res.TurnsCompleted = turns

	return nil
}

func main() {
	// Define the address and port the server will listen on
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	gameLife := new(GameOfLifeOperations)
	err := rpc.Register(gameLife)
	if err != nil {
		log.Fatalf("Error registering GameOfLifeOperations: %v", err)
	}

	// Start the listener
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		log.Fatalf("Error starting listener: %v", err)
	}
	defer listener.Close()

	log.Printf("Server is listening on port %s...\n", *pAddr)

	// Accept incoming connections and handle RPC requests
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue // Skip the current connection and continue to listen for new ones
		}

		// Serve the connection using RPC
		log.Printf("Connected! connection: %v", conn)
		go rpc.ServeConn(conn)
	}
}

func calculateNextState(workerNumber int, world [][]byte, threadCount int) [][]byte {
	// Create a new grid to store the next state of the world
	size := len(world)
	nextWorld := make([][]byte, size)
	for i := range nextWorld {
		nextWorld[i] = make([]byte, size)
	}

	flippedCells := []util.Cell{}

	// Iterate over each cell in the grid
	for y := workerNumber * size / threadCount; y < (workerNumber+1)*size/threadCount; y++ {
		for x := 0; x < size; x++ {
			sum := 0

			// Iterate over the 3x3 neighborhood
			for i := -1; i <= 1; i++ {
				for j := -1; j <= 1; j++ {
					// Skip the center cell itself (y, x)
					if i == 0 && j == 0 {
						continue
					}

					// Calculate wrapped coordinates using modulo
					ny := (y + i + size) % size
					nx := (x + j + size) % size

					// Sum the neighbor value (wrapped around)
					if world[ny][nx] == 255 {
						sum++
					}
				}
			}

			// Apply the Game of Life rules to the current cell
			if world[y][x] == 255 { // Cell is alive
				if sum < 2 || sum > 3 {
					nextWorld[y][x] = 0 // Cell dies
					flipped := util.Cell{x, y}
					flippedCells = append(flippedCells, flipped)
				} else {
					nextWorld[y][x] = 255 // Cell stays alive
				}
			} else { // Cell is dead
				if sum == 3 {
					nextWorld[y][x] = 255 // Cell becomes alive
					flipped := util.Cell{x, y}
					flippedCells = append(flippedCells, flipped)
				} else {
					nextWorld[y][x] = 0 // Cell stays dead
				}
			}
		}
	}
	// Return the next state of the world
	return nextWorld

}
