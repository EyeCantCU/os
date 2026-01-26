describe('guacamole app', () => {
  it('Login', () => {
    cy.visit('/guacamole/')
    // Login fields are dynamically generated via guac-form directive
    cy.get('.login-form input[type="text"]', { timeout: 10000 }).should('be.visible').type('test')
    cy.get('.login-form input[type="password"]').type('password')
    // Submit button is input[type="submit"][name="login"] with class "login"
    cy.get('input[type="submit"][name="login"].login').click()
    cy.get('.home-view', { timeout: 10000 }).should('be.visible')
    cy.get('#section-header-all-connections').should('be.visible')
  })
})

