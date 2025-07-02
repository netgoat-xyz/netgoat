import startNode from "./lib/raftNode.js";

export function initRaft(
  nodeId = "node1",
  port = 3000,
  peers = [],
  shardManagerUrl = "http://localhost:4000/move-shards"
) {
  process.env.NODE_ID = nodeId;
  process.env.PORT = port;
  process.env.PEERS = peers.join(",");
  process.env.SHARD_MANAGER_URL = shardManagerUrl;

  startNode({ nodeId, port, peers, shardManagerURL: shardManagerUrl }); // ✅ FIXED
}