describe('Kafbat UI FIPS', () => {
  it('loads the main UI', () => {
    cy.visit('https://localhost:8443/', { failOnStatusCode: false })

    // Check that the title is present
    cy.title().should('contain', 'Kafbat UI')

    // Check that the React app is loaded
    cy.get('#root', { timeout: 10000 }).should('exist')
  })

  it('has navigation buttons', () => {
    // Check that key navigation elements are present
    cy.visit('https://localhost:8443/', { failOnStatusCode: false })
    cy.contains('Topics', { timeout: 10000 }).should('exist')
    cy.contains('Consumers').should('exist')
  })

  it('API endpoints are accessible', () => {
    // Test that the API is responding
    cy.request({
      url: 'https://localhost:8443/api/clusters',
      failOnStatusCode: false
    }).then((response) => {
      expect(response.status).to.eq(200)
      expect(response.body).to.be.an('array')
    })
  })
})