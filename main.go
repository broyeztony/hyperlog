package main

import (
	"context"
	"encoding/json"
	"log"
	"math/big"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/syndtr/goleveldb/leveldb"
)

const (
	startBlock = 7817858 // first block involving 0x761d53b47334bee6612c0bd1467fb881435375b2 in a tx
	endBlock   = 7920102 // latest block on sepolia at testing time
	blockChunk = 10000
	topic      = "0x3e54d0825ed78523037d00a81759237eb436ce774bd546993ee67a1b67b6e766"
	address    = "0x761d53b47334bee6612c0bd1467fb881435375b2"
)

type Endpoint struct {
	URL     string
	Latency time.Duration
}

type LogEntry struct {
	BlockHash       common.Hash
	TransactionHash common.Hash
	Data            []byte
	BlockNumber     uint64
	Index           uint
}

type LogBlockInfo struct {
	Time             uint64
	Data             []byte
	TransactionsRoot common.Hash
	StateRoot        common.Hash
	ReceiptsRoot     common.Hash
	BlockParentHash  common.Hash
}

var endpoints = []Endpoint{ // fake endpoints
	{URL: "http://sepolia-geth-archive-node.org", Latency: 0},
	{URL: "http://sepolia-reth-archive-node.org", Latency: 0},
	{URL: "http://sepolia-erigon-archive-node.org/", Latency: 0},
}

var logsQueue = make(chan LogEntry, 100)
var blockInfoList = []LogBlockInfo{}
var blockInfoListMutex sync.Mutex
var db *leveldb.DB
var index uint64 = 0
var indexMutex sync.Mutex
var sepoliaChainId *big.Int

func main() {
	// Open log file
	logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	// Set log output to file
	log.SetOutput(logFile)

	// Set the sepolia chaindId
	sepoliaChainId = new(big.Int)
	sepoliaChainId.SetString("aa36a7", 16)

	// Initialize LevelDB
	db, err = leveldb.OpenFile("blockinfo.db", nil)
	if err != nil {
		log.Fatalf("Failed to open LevelDB: %v", err)
	}
	defer db.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go publisher(&wg)
	go subscriber(&wg)
	wg.Wait()
}

func publisher(wg *sync.WaitGroup) {
	defer wg.Done()

	for start := startBlock; start <= endBlock; start += blockChunk {
		end := start + blockChunk - 1
		if end > endBlock {
			end = endBlock
		}

		log.Printf("Querying logs from block %d to %d", start, end)
		queryLogs(start, end)
	}
	close(logsQueue)
	log.Println("Finished querying logs")
}

func queryLogs(start, end int) {
	client, endpoint := getFastestClient()
	defer client.Close()

	startTime := time.Now()
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(start)),
		ToBlock:   big.NewInt(int64(end)),
		Topics:    [][]common.Hash{{common.HexToHash(topic)}},
	}

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		log.Fatalf("Failed to filter logs: %v", err)
	}

	latency := time.Since(startTime)
	updateLatency(endpoint, latency)

	log.Printf("Fetched %d logs from %s in %s", len(logs), endpoint.URL, latency)
	putLogs(logs)
}

// Adding eth_getLogs logs to the channel queue
func putLogs(logs []types.Log) {

	for _, logEntry := range logs {
		logsQueue <- LogEntry{BlockHash: logEntry.BlockHash, Data: logEntry.Data, BlockNumber: logEntry.BlockNumber, TransactionHash: logEntry.TxHash, Index: logEntry.Index}
		log.Printf("Added log for block %d to queue", logEntry.BlockNumber)
	}
}

func subscriber(wg *sync.WaitGroup) {
	defer wg.Done()

	// read from channel queue. It will continue reading for as long as there are objects in it.
	for logEntry := range logsQueue {
		log.Printf("Processing log entry for block %d", logEntry.BlockNumber)
		processLogEntry(logEntry)
	}
	log.Println("Finished processing log entries")
}

func processLogEntry(logEntry LogEntry) {
	client, endpoint := getFastestClient()
	defer client.Close()

	startTime := time.Now()
	block, err := client.BlockByHash(context.Background(), logEntry.BlockHash)
	if err != nil {
		log.Fatalf("Failed to get block: %v", err)
	}

	latency := time.Since(startTime)
	updateLatency(endpoint, latency)

	log.Printf("Fetched block %s from %s in %s", logEntry.BlockHash.Hex(), endpoint.URL, latency)

	for _, tx := range block.Transactions() {
		if tx.Hash() == logEntry.TransactionHash {

			sender, err := types.Sender(types.NewLondonSigner(sepoliaChainId), tx)
			if err != nil {
				log.Fatal("Not able to retrieve Tx sender:", err)
				continue
			}

			if (tx.To() != nil && tx.To().Hex() == address) || (sender.Hex() == address) {
				blockInfo := LogBlockInfo{
					Time:             block.Time(),
					Data:             logEntry.Data,
					TransactionsRoot: block.TxHash(),
					StateRoot:        block.Root(),
					ReceiptsRoot:     block.ReceiptHash(),
					BlockParentHash:  block.ParentHash(),
				}
				log.Printf("Storing block info for log index %d", logEntry.BlockNumber)
				storeBlockInfo(blockInfo)
			}
		}
	}
}

// Add to LevelDb
func storeBlockInfo(logBlockInfo LogBlockInfo) {
	blockInfoListMutex.Lock()
	defer blockInfoListMutex.Unlock()

	data, err := json.Marshal(logBlockInfo)
	if err != nil {
		log.Fatalf("Failed to marshal block info: %v", err)
	}

	log.Printf("Serialized block info: %s", data)

	// Get the current index and increment it
	indexMutex.Lock()
	key := strconv.FormatUint(index, 10)
	index++
	indexMutex.Unlock()

	// Store the blockInfo in LevelDB
	err = db.Put([]byte(key), data, nil)
	if err != nil {
		log.Fatalf("Failed to store block info in LevelDB: %v", err)
	}
	log.Printf("Stored block info with key %s: %+v", key, logBlockInfo)
}

func getFastestClient() (*ethclient.Client, *Endpoint) {
	sortEndpointsByLatency()
	endpoint := &endpoints[0]
	client, err := ethclient.Dial(endpoint.URL)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	return client, endpoint
}

func updateLatency(endpoint *Endpoint, latency time.Duration) {
	for i := range endpoints {
		if endpoints[i].URL == endpoint.URL {
			endpoints[i].Latency = latency
			break
		}
	}
	sortEndpointsByLatency()
}

func sortEndpointsByLatency() {
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].Latency < endpoints[j].Latency
	})
	log.Println("Sorted endpoints by latency:", endpoints)
}
