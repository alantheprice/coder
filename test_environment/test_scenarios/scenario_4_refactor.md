# Test Scenario 4: Refactor Code

## Task Description
Refactor simple_server.go to use the configuration from config.json instead of hardcoded values.

## Specific Requirements
1. Create a Config struct that matches config.json structure
2. Load configuration at startup from config.json file
3. Use config values for server port instead of hardcoded ":8080"
4. Add error handling for missing/invalid config file
5. Update the `/status` endpoint to show loaded configuration
6. Maintain backward compatibility if config file is missing

## Expected Behavior
- Server should start using port from config.json (8080)
- Should gracefully handle missing config.json with defaults
- `/status` should show current configuration values
- Code should be cleaner and more maintainable

## Success Criteria
- [ ] Config struct is properly defined matching config.json
- [ ] Configuration is loaded at startup
- [ ] Server uses config.Server.Port instead of hardcoded port
- [ ] Proper error handling for file operations
- [ ] `/status` endpoint shows configuration information
- [ ] Code compiles and runs correctly
- [ ] Graceful fallback to defaults if config is missing

## Complexity Level: VERY HARD (Multi-step refactoring + integration)

## Expected Iterations: 10-15
## Expected Tool Calls: 15-25

## Common Failure Points
- Incorrect struct tags for JSON unmarshaling
- Poor error handling for file operations
- Breaking existing functionality during refactor
- Not handling missing config file gracefully
- Circular dependencies or import issues