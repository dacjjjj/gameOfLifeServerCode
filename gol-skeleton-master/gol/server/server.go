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

var tickRequest = make(chan struct{}, 1)
var currentTurns = make(chan int, 1)
var cellCount = make(chan int, 1)

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
	passedWorld := initReq.NextWorld
	passedTurns := initReq.Turns
	passedThreads := 4
	sizeOfWorld := len(passedWorld)
	responseChannels := make([]chan [][]uint8, passedThreads)
	for i := 0; i < passedThreads; i++ {
		responseChannels[i] = make(chan [][]uint8)
	}

	for localTurns := 0; localTurns < passedTurns; localTurns++ {
		log.Printf("im in the for loop")
		select {
		case <-tickRequest:
			log.Printf("ive recieved the request")
			currentTurns <- localTurns - 1 // Sample turn value
			cellCount <- len(calculateAliveCells(passedWorld))
		default:
		}
		newWorld := make([][]byte, sizeOfWorld)
		for j := range newWorld {
			newWorld[j] = make([]byte, sizeOfWorld)
		}

		for k, workerAddr := range workerNodes {
			if workerAddr == "" {
				log.Printf("Worker address is empty. Skipping this worker.\n")
				continue
			}

			wg.Add(1)
			go func(workerAddr string, index int) {
				defer wg.Done()
				client2, err1 := rpc.Dial("tcp", workerAddr)
				//fmt.Printf("worker number: %s", i)
				if err1 != nil {
					return
				}
				req := stubs.Request{
					WorkerNumber: k,
					NextWorld:    passedWorld,
					Turns:        passedTurns,
					ThreadCount:  passedThreads,
				}
				res := new(stubs.Response)
				client2.Call(stubs.StartWorker, req, res)
				responseChannels[k] <- res.FinalWorld

			}(workerAddr, k)
		}

		for j := 0; j < passedThreads; j++ {
			if j < len(responseChannels) {

				receivedWorld := <-responseChannels[localTurns]
				// Calculate the vertical range this worker's piece should fill in newWorld
				for y := j * sizeOfWorld / passedThreads; y < (j+1)*sizeOfWorld/passedThreads; y++ {
					for x := 0; x < sizeOfWorld; x++ {
						newWorld[y][x] = receivedWorld[y][x]
					}
				}
			}
		}

		passedWorld = newWorld

	}

	go func() {
		wg.Wait()
		for _, ch := range responseChannels {
			close(ch) // Close each channel after all workers are done

		}
	}()

	finalRes.FinalWorld = passedWorld
	finalRes.AliveCells = calculateAliveCells(passedWorld)
	finalRes.TurnsCompleted = initReq.Turns
	return
}

func (g *GolMasterRunner) TickTime(aliveRequest *stubs.AliveRequest, aliveCellResponse *stubs.AliveResponse) (err error) {
	log.Printf("i made it to here0\n", aliveRequest)
	tickRequest <- struct{}{}
	log.Printf("i made it to here1\n")
	currentTurn2 := <-currentTurns
	log.Printf("i made it to here2\n")
	cellCount2 := <-cellCount
	log.Printf("i made it to here3\n")
	aliveCellResponse.AliveCellCount = cellCount2
	aliveCellResponse.CurrentTurns = currentTurn2
	log.Printf("i made it to here4\n", currentTurn2, cellCount2)
	return
}

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
