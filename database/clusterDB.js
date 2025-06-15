import { randomUUID } from "node:crypto";
import SingleFileDocDB from "./singleFileDB.js";

const db = new SingleFileDocDB("./db.json");
await db.init();

export async function write(collection, doc) {
  if (!doc._id) doc._id = randomUUID();
  return db.write(collection, doc);
}

export async function findOne(collection, filterFn) {
  const results = await db.find(collection, filterFn);
  return results[0] || null;
}
