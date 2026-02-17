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

  // Node.js 20.12+ added crypto.hash() which bypasses createHash
  if (typeof crypto.hash === 'function') {
    const originalHash = crypto.hash;
    crypto.hash = function(algorithm, ...rest) {
      if (algorithm === 'md5') {
        algorithm = 'sha1';
      }
      return originalHash(algorithm, ...rest);
    };
  }
}

monkeyPatch();

// Update this path to match your circumstances
require('/usr/bin/pnpm');