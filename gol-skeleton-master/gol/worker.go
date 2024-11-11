package main

import (
	"errors"
	"flag"
	"log"
	"net"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
)

type GameOfLifeOperations struct{}

func main() {
	// Define the address and port the server will listen on
	pAddr := flag.String("port", "8040", "Port to listen on")
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
			continue //
		}

		log.Printf("Connected! connection: %v", conn)
		go rpc.ServeConn(conn)
	}
}

func (g *GameOfLifeOperations) ProcessGameOfLife(req stubs.Request, res *stubs.Response) (err error) {
	if req.NextWorld == nil {
		err = errors.New("no final board received")
		return
	}

	nextWorld := req.NextWorld
	turns := req.Turns
	workerId := req.WorkerNumber
	threadCount := req.ThreadCount

	nextWorld = calculateNextState(workerId, nextWorld, threadCount)

	res.FinalWorld = nextWorld
	res.TurnsCompleted = turns
	return
}

func calculateNextState(workerNumber int, world [][]byte, threadCount int) [][]byte {
	size := len(world)
	nextWorld := make([][]byte, size)
	for i := range nextWorld {
		nextWorld[i] = make([]byte, size)
	}

	for y := workerNumber * size / threadCount; y < (workerNumber+1)*size/threadCount; y++ {
		for x := 0; x < size; x++ {
			sum := 0

			for i := -1; i <= 1; i++ {
				for j := -1; j <= 1; j++ {
					if i == 0 && j == 0 {
						continue
					}

					ny := (y + i + size) % size
					nx := (x + j + size) % size

					if world[ny][nx] == 255 {
						sum++
					}
				}
			}

			if world[y][x] == 255 { //cell is alive
				if sum < 2 || sum > 3 {
					nextWorld[y][x] = 0 //cell dies
				} else {
					nextWorld[y][x] = 255 //cell stays alive
				}
			} else { //cell is dead
				if sum == 3 {
					nextWorld[y][x] = 255 //cell becomes alive
				} else {
					nextWorld[y][x] = 0 //cell stays dead
				}
			}
		}
	}
	return nextWorld

}
