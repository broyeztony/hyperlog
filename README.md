# My approach to the problem

First thing I did was to check https://sepolia.etherscan.io/ for the presumably contract address `0x761d53b47334bee6612c0bd1467fb881435375b2`

I noticed it was not a contract but an EOA. 
I looked up for the first block (`4789178`) involving that address in a transaction, i.e https://sepolia.etherscan.io/tx/0x9844f923258c54bd17700af423089287345517c4fc0647ba8a5b449020698229

Since `0x761d53b47334bee6612c0bd1467fb881435375b2` is an EOA and since there is no block
referencing that address before `4789178`, we can simplify the problem as:
 - index the logs + block information which involves `0x761d53b47334bee6612c0bd1467fb881435375b2` as a source or destination address, starting from block `4789178`

Also, we have an additional piece of information to decrease again the complexity of the problem, as we
are looking for logs where `0x3e54d0825ed78523037d00a81759237eb436ce774bd546993ee67a1b67b6e766` is a topic.

My approach involves a publisher / subscriber pattern using 2 goroutines.

The publisher is a routine that fetch logs filtered by the topic `0x3e54d0825ed78523037d00a81759237eb436ce774bd546993ee67a1b67b6e766` and adds then to a queue.
It queries logs by chunks (of 10,000) to avoid incuring endpoints's timeout (eth_getLogs is a resource-heavy API) and adds them
to the queue.

The subscriber is a routine that reads from the queue and for each log entry, queries the corresponding block
with its transactions. Then, we lookup the transactions in the fetched block and 
see if any of them references the target address either as a source (`from` field) or 
a destination (`to` field)

In the affirmative, we store the log's data along with the corresponding block's time and block's parentHash in LevelDB.

The indexer spread the load over 3 endpoints (Reth, Geth, Erigon) and always use the last fastest endpoint for the next queries.
