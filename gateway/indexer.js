
/*
start block: 4789178
end block: 7833217
fetch all logs for topic 0x3e54d0825ed78523037d00a81759237eb436ce774bd546993ee67a1b67b6e766 between start and end blocks by chunks of 10000 blocks
i.e, we query [4789178, 4799178], [4799178, 4809178], etc
spread the load over this 3 endpoints [
  { url: 'http://sepolia-erigon-archive-blb.ethereum-network-1-dev-us-east-1.eks-dev.infura.org/', latency: 0 }
  { url: 'http://sepolia-geth-archive-blb.ethereum-network-1-dev-us-east-1.eks-dev.infura.org/', latency: 0 }
  { url: 'http://sepolia-reth-archive-blb.ethereum-network-1-dev-us-east-1.eks-dev.infura.org/', latency: 0 }
]
each endpoint has an initial latency of 0. For each request, the latency must be measured and the corresponding endpoint's latency field must be updated.
then the endpoint are sorted by latency, so that the first endpoint in the list is always the fastest.
It is always the endpoint with the lowest latency that must be queried.

for each result, reduce by blockHash and we aggregate by keeping another field `transactionsHashes` which maps transactionHash to logIndex
append the reduced result to a FIFO queue called 'logs'.
The above process is the publisher

Another process (the subscriber) will read from the logs's FIFO.
It will query the corresponding block information using the blockHash. Again, we select the fastest endpoint and keep this list up-to-date

for each queried block, we check if there are transactions.
If there are, we look for transactions involving the address `0x761d53b47334bee6612c0bd1467fb881435375b2`
(either in the `from` or the `to` field)
if there are, we create an object with fields:
 - time: the block's timestamp
 - transactionsRoot
 - stateRoot
 - receiptsRoot
 - blockParentHash
and we store this object in an in-memory list
*/


