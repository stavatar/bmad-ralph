### [HIGH] Missing error assertion

The test checks `err != nil` but never verifies message content.

### [MEDIUM] Stale doc comment

Doc comment says "returns nil" but function returns ErrNoRecovery.

### [LOW] Unused test fixture

testdata/old-fixture.golden is never referenced.

### [CRITICAL] SQL injection in query builder

User input passed directly to query without escaping.

### [MEDIUM] DRY violation in test helpers

Same closure copied 3 times across test functions.
