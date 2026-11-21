#!/usr/bin/env node
import { startServer } from '../src/index.js';

startServer().catch((err) => {
  console.error(err instanceof Error ? err.message : err);
  process.exit(1);
});
