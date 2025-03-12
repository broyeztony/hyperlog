import * as fs from 'fs'
import axios  from 'axios'
import { setTimeout } from 'node:timers/promises'

// Read the transaction hashes from the file
const hashes = fs.readFileSync('hashes.tsv', 'utf-8').split('\n').filter(Boolean);

// (fake) Ethereum node URLs
const nodeUrls = [
  'http://sepolia-erigon-archive-node/',
  'http://sepolia-geth-archive-blb.ethereum-network-1-dev-us-east-1.eks-dev.infura.org/',
  'http://sepolia-reth-archive-blb.ethereum-network-1-dev-us-east-1.eks-dev.infura.org/'
]

const outputFile = 'blocks.json';

// Function to query block information for a given transaction hash
async function queryBlock(blockHash) {
  try {

    // Query the block information using the block number
    const nodeUrl = nodeUrls[Math.floor(Math.random() * nodeUrls.length)]
    const blockResponse = await axios.post(nodeUrl, {
      jsonrpc: '2.0',
      method: 'eth_getBlockByHash',
      params: [blockHash, true],
      id: 1
    });

    const block = blockResponse.data.result;
    if (!block) {
      console.log(`Block not found for block number: ${blockHash}`);
      return;
    }

    if (block.transactions.length > 0) {
      const interestingTXs = block.transactions.filter(tx => {
        return tx.from === "0x761d53b47334bee6612c0bd1467fb881435375b2" || tx.to === "0x761d53b47334bee6612c0bd1467fb881435375b2"
      })

      if (interestingTXs.length > 0) {

        const blockData = {
          blockHash: block.hash,
          time: block.timestamp,
          transactionsRoot: block.transactionsRoot,
          stateRoot: block.stateRoot,
          receiptsRoot: block.receiptsRoot,
          blockParentHash: block.parentHash,
          extraData: block.extraData,
          transactions: interestingTXs.map(tx => {
            return {
              from: tx.from,
              to: tx.to,
              hash: tx.hash
            }
          })
        }

        fs.appendFileSync(outputFile, JSON.stringify(blockData, null, 2) + ',\n', 'utf-8');
        console.log(`Block information for block hash ${blockHash} appended to ${outputFile}`);
      } else {
        console.log(`No transactions relevant to 0x761d53b47334bee6612c0bd1467fb881435375b2 in`, block.hash);
      }
    } else {
      console.log(`No transactions for block`, block.hash);
    }

  } catch (error) {
    console.error(`Error querying block for block hash ${blockHash}:`, error.message);
  }
}

async function processHashes(hashes) {
  for (const hash of hashes) {
    await queryBlock(hash);
    await setTimeout(1)
  }
}

// Process the transaction hashes with the specified delay
processHashes(hashes);
