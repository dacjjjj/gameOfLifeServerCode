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

type golMasterRunner struct{}

func main() {
	// List of worker node addresses (replace with IPs or DNS of your EC2 instances)
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&golMasterRunner{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)

}

func (g *golMasterRunner) masterStart(initReq stubs.InitialRequest, finalRes *stubs.FinalResponse) (err error) {

	workerNodes := []string{
		"ip-172-31-86-15.ec2.internal.us-east-1b.compute.internal:8080", // Replace with worker IP/DNS
		//"ip-172-31-87-196.ec2.internal.us-east-1b.compute.internal:8080",
		//"ip-172-31-88-186.ec2.internal.us-east-1b.compute.internal:8080",
		//"ip-172-31-82-182.ec2.internal.us-east-1b.compute.internal:8080",
		//"localhost:8081", // Worker 1
		//"localhost:8082", // Worker 2
		//"localhost:8083", // Worker 3
		//"localhost:8084",
	}

	var wg sync.WaitGroup
	responses := make([]stubs.Response, len(workerNodes)) // Slice to collect responses
	passedWorld := initReq.NextWorld
	passedTurns := initReq.Turns
	passedThreads := initReq.ThreadCount
	sizeOfWorld := len(passedWorld)
	// Channel to safely collect responses from goroutines
	responseChannel := make(chan stubs.Response)

	// Define tasks and assign worker IDs
	for i, workerAddr := range workerNodes {
		wg.Add(1)
		// Create the request with a unique worker ID
		req := stubs.Request{
			WorkerNumber: i,           // Unique ID for each worker (1, 2, 3, etc.)
			NextWorld:    passedWorld, // Your game world data goes here
			Turns:        passedTurns,
			ThreadCount:  passedThreads, // Number of turns or other task parameters
		}

		// Call the worker's processGameOfLife method remotely concurrently
		go func(workerAddr string, req stubs.Request, index int) {
			defer wg.Done() // Decrement the counter when the goroutine finishes
			res := callWorker(workerAddr, req)
			// Send the response to the channel for aggregation
			responseChannel <- res
		}(workerAddr, req, i)
	}

	// Wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(responseChannel) // Close the channel when done collecting responses
	}()

	// Collect all responses from the channel
	for res := range responseChannel {
		// Append each worker's response
		responses = append(responses, res)
	}

	for i := 0; i < passedThreads; i++ {
		// Get the chunk of the world for this worker (response)
		receivedWorld := responses[i].FinalWorld

		// Calculate the vertical range this worker's piece should fill in newWorld
		for y := i * sizeOfWorld / passedThreads; y < (i+1)*sizeOfWorld/passedThreads; y++ {
			for x := 0; x < sizeOfWorld; x++ {
				// Assign the piece of the received world to the corresponding part in the final world
				passedWorld[y][x] = receivedWorld[y][x]
			}
		}
	}
	FinalResponseWorld := passedWorld
	finalRes.FinalWorld = FinalResponseWorld
	finalRes.AliveCells = calculateAliveCells(FinalResponseWorld)
	finalRes.TurnsCompleted = initReq.Turns

	return
}

// callWorker sends a request to a worker and receives a response
func callWorker(workerAddr string, req stubs.Request) stubs.Response {
	// Connect to the worker via RPC
	client, err := rpc.Dial("tcp", workerAddr)
	if err != nil {
		log.Printf("Error connecting to worker: %v\n", err)
		return stubs.Response{} // Return an empty response on error
	}
	defer client.Close()

	// Prepare a response to receive from the worker
	var res stubs.Response

	// Call the worker's processGameOfLife method (this is the remote procedure call)
	err = client.Call("GameOfLifeOperations.processGameOfLife", &req, &res)
	if err != nil {
		log.Printf("Error calling worker's processGameOfLife: %v\n", err)
		return stubs.Response{} // Return an empty response on error
	}

	// Return the response from the worker
	return res
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
