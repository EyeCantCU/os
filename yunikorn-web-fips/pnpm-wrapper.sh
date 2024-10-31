#!/usr/bin/env node
const crypto = require('node:crypto');
const path = require('node:path');

function monkeyPatch() {
  const originalCreateHash = crypto.createHash;

  function createHashInterceptMD5(...argv) {
    if (argv[0] === 'md5') {
      argv[0] = 'sha1';
    }
    return originalCreateHash(...argv);
  }

  crypto.createHash = createHashInterceptMD5;
}

monkeyPatch();

// Update this path to match your circumstances
require('/usr/bin/pnpm');