# Test Scenario 3: Add Unit Tests

## Task Description
Create comprehensive unit tests for all functions in utils.go.

## Specific Requirements
1. Create a new file `utils_test.go` 
2. Test `FormatName()` with at least 3 test cases including edge cases
3. Test `CalculateTotal()` with valid inputs, invalid inputs, and edge cases
4. Test `ValidateEmail()` with valid emails, invalid emails, and edge cases
5. Use proper Go testing conventions (`func TestXxx(t *testing.T)`)
6. Include table-driven tests where appropriate

## Expected Behavior
- All tests should pass with `go test`
- Should cover normal cases, edge cases, and error conditions
- Should follow Go testing best practices

## Success Criteria
- [ ] File `utils_test.go` is created
- [ ] `TestFormatName` tests multiple cases including empty string
- [ ] `TestCalculateTotal` tests valid prices, invalid formats, empty slices
- [ ] `TestValidateEmail` tests valid/invalid emails, empty strings
- [ ] All tests pass when running `go test`
- [ ] Tests use `t.Errorf` or similar for assertions
- [ ] Tests have descriptive names and error messages

## Complexity Level: HARD (Requires understanding existing code + comprehensive testing)

## Expected Iterations: 8-12
## Expected Tool Calls: 12-18

## Common Failure Points
- Missing import statements (`testing` package)
- Incorrect test function signatures
- Not handling edge cases properly
- Tests that don't actually test the right behavior
- Missing error checking in tests