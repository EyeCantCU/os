describe('VictoriaLogs vmui', () => {
  it('loads the vmui page', () => {
    cy.visit('/select/vmui')
    cy.title().should('eq', 'UI for VictoriaLogs')
  })

  it('has the main app container', () => {
    cy.visit('/select/vmui')
    cy.get('#root').should('exist')
  })

  it('loads required JavaScript assets', () => {
    cy.visit('/select/vmui')
    cy.document().then((doc) => {
      const scripts = doc.querySelectorAll('script[src]')
      expect(scripts.length).to.be.greaterThan(0)
    })
  })

  it('renders the application UI', () => {
    cy.visit('/select/vmui')
    cy.get('#root').children().should('have.length.greaterThan', 0)
  })
})
