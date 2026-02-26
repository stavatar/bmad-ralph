# Story: User Login Authentication

As a registered user,
I want to authenticate with email and password,
so that I can access protected resources.

## Acceptance Criteria

### AC-1: Login Endpoint
Given a valid email and password,
When the user submits login credentials,
Then the system returns a 200 status with an access token.

### AC-2: Input Validation
Given invalid or missing credentials,
When the user submits the login form,
Then the system returns a 400 status with descriptive error messages.

### AC-3: JWT Response
Given a successful authentication,
When the login endpoint responds,
Then the response body contains a valid JWT token with user claims.
