const { defineConfig } = require('cypress')
module.exports = defineConfig({
  e2e: {
    baseUrl: 'http://localhost:9428',
    supportFile: false,
    video: false,
    screenshotOnRunFailure: false,
  },
})
