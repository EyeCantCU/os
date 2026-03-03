describe('Keep UI', () => {
  it('loads the signin page', () => {
    cy.visit('/signin')
    cy.get('html').should('exist')
  })

  it('renders the signin page content', () => {
    cy.visit('/signin')
    cy.get('body').children().should('have.length.greaterThan', 0)
  })

  it('loads required JavaScript assets', () => {
    cy.visit('/signin')
    cy.document().then((doc) => {
      const scripts = doc.querySelectorAll('script[src]')
      expect(scripts.length).to.be.greaterThan(0)
    })
  })

  it('serves the healthcheck endpoint', () => {
    cy.request('/api/healthcheck').its('status').should('eq', 200)
  })

  it('serves public assets', () => {
    cy.request('/keep.svg').its('status').should('eq', 200)
  })

  it('completes NO_AUTH login flow', () => {
    cy.visit('/signin')
    // NO_AUTH auto-redirects through NextAuth; wait for URL to leave /signin
    cy.url().should('not.include', '/signin', { timeout: 30000 })
    // Verify the authenticated page rendered content
    cy.get('body').children().should('have.length.greaterThan', 0)
  })
})
