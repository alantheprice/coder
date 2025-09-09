# Systematic Testing Scoring Criteria

## Automated Metrics (Objective)

### 1. Completion Metrics (40 points)
- **Task Completed Successfully**: 25 points (binary: did it work?)
- **Code Compiles**: 10 points (binary: no syntax errors)
- **Meets All Requirements**: 5 points (checklist completion %)

### 2. Efficiency Metrics (30 points)
- **Iteration Count**: 15 points (fewer iterations = higher score)
  - 1-5 iterations: 15 points
  - 6-10 iterations: 10 points  
  - 11-15 iterations: 5 points
  - 16+ iterations: 0 points
- **Tool Call Accuracy**: 10 points (valid calls / total calls)
- **Token Efficiency**: 5 points (lower tokens for same result = higher score)

### 3. Technical Quality (20 points)
- **Code Quality**: 10 points (follows patterns, readable, maintainable)
- **Error Handling**: 5 points (graceful failure, proper validation)
- **Best Practices**: 5 points (follows Go/language conventions)

### 4. Strategic Thinking (10 points)
- **Progressive Approach**: 5 points (logical step sequence)
- **Error Recovery**: 3 points (adapts when things fail)
- **Context Awareness**: 2 points (learns from previous attempts)

## Scoring Rubric

### Total Score: 100 points

**90-100**: Excellent - Professional quality, efficient execution
**80-89**: Good - Completes task well with minor issues
**70-79**: Satisfactory - Completes task but inefficient or with problems
**60-69**: Poor - Partial completion or major issues
**0-59**: Fail - Does not complete task or produces broken code

## Automated Checks

### Binary Checks (Pass/Fail)
- Does code compile? (`go build` succeeds)
- Do tests pass? (`go test` succeeds) 
- Does server start? (for server scenarios)
- Are all required functions/endpoints present?

### Measurable Checks
- Line count of changes (scope of work)
- Number of files created/modified
- Cyclomatic complexity (code quality)
- Token count and cost

### Pattern Checks
- Uses proper Go conventions (gofmt, naming)
- Includes proper error handling
- Has appropriate comments/documentation
- Follows existing code patterns