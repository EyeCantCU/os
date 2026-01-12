const { defineConfig } = require('cypress')

module.exports = defineConfig({
  e2e: {
    baseUrl: 'https://localhost:8443',
    supportFile: false,
    chromeWebSecurity: false, // Allow testing HTTPS with self-signed certs
  },
})