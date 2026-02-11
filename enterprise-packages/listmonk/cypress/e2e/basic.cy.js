describe('listmonk app', () => {
  beforeEach(() => {
    // Cypress starts out with a blank slate for each test
    // so we must tell it to visit our website with the `cy.visit()` command.
    // Since we want to visit the same URL at the start of all our tests,
    // we include it in our beforeEach function so that it runs before each test
    cy.visit('http://localhost:9000/')
  })

    it('Login', () => {
        cy.visit('/admin/login')
        cy.get("#username")
            .type("admin");
        cy.get("#password")
            .type("password");
        cy.get('form').submit();
        // should be logged in now
        cy.visit('/admin/user/profile');
        // check profile page title
        cy.get('[class=title]').should("have.text", " @admin ");
    });
});
