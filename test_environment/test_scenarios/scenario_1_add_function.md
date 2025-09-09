# Test Scenario 1: Add Simple Function

## Task Description
Add a new function called `GetServerInfo()` to simple_server.go that returns server information as a JSON response.

## Specific Requirements
1. Function should be called `GetServerInfo()`
2. Should return a struct with fields: `Name`, `Version`, `Port`
3. Values should come from a new `/info` HTTP endpoint
4. Should set proper Content-Type header as `application/json`
5. Should be properly integrated into the existing server

## Expected Behavior
- GET `/info` should return: `{"name":"SimpleServer","version":"1.0.0","port":8080}`
- Should compile without errors
- Should follow existing code patterns

## Success Criteria
- [ ] Function `GetServerInfo()` exists and is properly structured
- [ ] `/info` endpoint is registered in main()
- [ ] Response is valid JSON with correct Content-Type
- [ ] Code compiles successfully
- [ ] Follows existing code style

## Complexity Level: EASY (Baseline test)

## Expected Iterations: 3-5
## Expected Tool Calls: 5-8

## Common Failure Points
- Missing Content-Type header
- Malformed JSON structure
- Not registering the endpoint in main()
- Wrong function signature