# Test Scenario 2: Fix Known Bug

## Task Description
Fix the JSON formatting bug in the `/status` endpoint in simple_server.go.

## Specific Requirements
1. Fix the malformed JSON in `statusHandler` (missing comma)
2. Add proper Content-Type header `application/json`
3. Make the JSON properly formatted and valid
4. Add an actual uptime calculation or placeholder
5. Ensure the response is syntactically correct

## Expected Behavior
- GET `/status` should return valid JSON: `{"status":"running","uptime":"5m30s"}`
- Should set Content-Type: application/json header
- Should compile and run without errors

## Success Criteria
- [ ] JSON syntax is fixed (comma between fields)
- [ ] Content-Type header is set correctly
- [ ] Response is valid, parseable JSON
- [ ] Code compiles successfully
- [ ] Uptime field has a reasonable value

## Complexity Level: MEDIUM (Requires analysis + fixing)

## Expected Iterations: 4-6
## Expected Tool Calls: 6-10

## Common Failure Points
- Only fixing syntax but missing Content-Type header
- Over-engineering the uptime calculation
- Breaking other parts of the function
- Not testing the fix properly