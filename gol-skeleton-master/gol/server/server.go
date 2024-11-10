package main

import (
	"flag"
	"log"
	"net"
	"net/rpc"
	"sync"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GolMasterRunner struct{}

func main() {
	// List of worker node addresses (replace with IPs or DNS of your EC2 instances)
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	golMaster := new(GolMasterRunner)
	err := rpc.Register(golMaster)
	if err != nil {
		log.Fatalf("Error registering GolMasterRunner: %v", err)
	}

	// Start the listener
	listener, err := net.Listen("tcp", "0.0.0.0:"+*pAddr) // Bind to all interfaces
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

type WorkerResponse struct {
	FinalWorld     [][]byte // The updated world from the worker
	WorkerNumber   int
	TurnsCompleted int // The number of turns completed by the worker
}

func (g *GolMasterRunner) MasterStart(initReq stubs.InitialRequest, finalRes *stubs.FinalResponse) (err error) {
	workerNodes := []string{
		"ec2-52-204-0-223.compute-1.amazonaws.com:8040",
		"ec2-52-205-110-39.compute-1.amazonaws.com:8040",
		"ec2-34-198-86-17.compute-1.amazonaws.com:8040",
		"ec2-34-199-220-125.compute-1.amazonaws.com:8040",
	}

	var wg sync.WaitGroup
	responses := []WorkerResponse{} // Change responses to hold WorkerResponse structs
	passedWorld := initReq.NextWorld
	passedTurns := initReq.Turns
	passedThreads := initReq.ThreadCount
	sizeOfWorld := len(passedWorld)
	responseChannel := make(chan WorkerResponse)

	for i, workerAddr := range workerNodes {
		if workerAddr == "" {
			log.Printf("Worker address is empty. Skipping this worker.\n")
			continue
		}

		wg.Add(1)
		req := stubs.Request{
			WorkerNumber: i,
			NextWorld:    passedWorld,
			Turns:        passedTurns,
			ThreadCount:  4,
		}
		log.Printf("stuff.\n", passedTurns, passedThreads)

		go func(workerAddr string, req stubs.Request, index int) {
			defer wg.Done()
			log.Printf("workerAddress: %v\n", workerAddr)
			res := callWorker(workerAddr, req)
			responseChannel <- res
		}(workerAddr, req, i)
	}

	go func() {
		wg.Wait()
		close(responseChannel)
	}()

	for res := range responseChannel {
		if res.FinalWorld != nil {
			responses = append(responses, res)
		} else {
			log.Printf("Received invalid response from worker: %v\n", res)
		}
	}

	for i := 0; i < 3; i++ {
		if i < len(responses) {
			receivedWorld := responses[i].FinalWorld

			// Calculate the vertical range this worker's piece should fill in newWorld
			for y := i * sizeOfWorld / 4; y < (i+1)*sizeOfWorld/4; y++ {
				for x := 0; x < sizeOfWorld; x++ {
					passedWorld[y][x] = receivedWorld[y][x]
				}
			}
		}
	}

	FinalResponseWorld := passedWorld
	finalRes.FinalWorld = FinalResponseWorld
	finalRes.AliveCells = calculateAliveCells(FinalResponseWorld)
	finalRes.TurnsCompleted = initReq.Turns
	log.Printf("passingBack from server.\n", FinalResponseWorld, calculateAliveCells(FinalResponseWorld), initReq.Turns)
	return nil
}

// callWorker sends a request to a worker and receives a response
func callWorker(workerAddr string, req stubs.Request) WorkerResponse {
	// Connect to the worker via RPC
	log.Printf("workerAddress now in callworker: %v\n", workerAddr)
	client, err := rpc.Dial("tcp", workerAddr)
	if err != nil {
		log.Printf("Error connecting to worker %v: %v\n", workerAddr, err)
		return WorkerResponse{} // Return an empty WorkerResponse on connection failure
	}
	defer client.Close()

	// Prepare a response to receive from the worker
	res := new(stubs.Response)

	// Call the worker's processGameOfLife method (this is the remote procedure call)
	err = client.Call(stubs.StartWorker, req, res)
	if err != nil {
		log.Printf("Error calling worker's processGameOfLife: %v\n", err)
		return WorkerResponse{} // Return an empty WorkerResponse on error
	}

	// Map stubs.Response to WorkerResponse

	log.Printf("connected: %v\n", workerAddr, res.TurnsCompleted, res.WorkerNumber, "de ", req.Turns, req.WorkerNumber)
	return WorkerResponse{
		FinalWorld:     res.FinalWorld,
		WorkerNumber:   res.WorkerNumber,
		TurnsCompleted: res.TurnsCompleted,
	}
}

// calculateAliveCells counts the number of alive cells in the world (stub implementation)

func calculateAliveCells(world [][]byte) []util.Cell {
	size := len(world)
	aliveCollection := []util.Cell{}
	for y := 0; y < size; y++ { // Iterate over rows (height)
		for x := 0; x < size; x++ { // Iterate over columns (width)
			if world[y][x] == 255 { // Access as world[row][column] or world[y][x]
				alive := util.Cell{x, y}
				aliveCollection = append(aliveCollection, alive)
			}
		}
	}
	return aliveCollection
}
