# Story: User Registration

As a new user,
I want to register an account,
so that I can access the application.

## Acceptance Criteria

### AC-1: Registration Endpoint
Given valid registration data,
When the user submits the registration form,
Then a new account is created and a confirmation email is sent.

### AC-2: Duplicate Prevention
Given an email already in use,
When the user attempts to register,
Then the system returns a conflict error without creating a duplicate.
