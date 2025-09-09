# Systematic Prompt Testing Framework

A controlled testing environment for measuring and improving AI agent prompt engineering.

## Quick Start

```bash
# Run a single test
./run_test.sh v1_current scenario_1_add_function

# Run with debug output
./run_test.sh v1_current scenario_1_add_function --debug

# Compare results across prompt versions
./compare_results.sh scenario_1_add_function
```

## Test Scenarios

### Scenario 1: Add Simple Function (Easy)
- **Goal**: Add a new HTTP endpoint with JSON response
- **Expected**: 3-5 iterations, basic Go patterns
- **Tests**: Code structure, compilation, HTTP routing

### Scenario 2: Fix Known Bug (Medium) 
- **Goal**: Fix malformed JSON and missing headers
- **Expected**: 4-6 iterations, analysis + correction
- **Tests**: Bug identification, proper fix, testing

### Scenario 3: Add Unit Tests (Hard)
- **Goal**: Create comprehensive test suite
- **Expected**: 8-12 iterations, understand existing code
- **Tests**: Test coverage, edge cases, Go testing conventions

### Scenario 4: Refactor Code (Very Hard)
- **Goal**: Extract config, maintain backward compatibility  
- **Expected**: 10-15 iterations, complex multi-step changes
- **Tests**: Architecture, error handling, integration

## Prompt Versions

### v1_current (Baseline)
Current production prompt - verbose, complex tool instructions

### v2_structured  
Phase-based approach with explicit planning steps

### v3_simplified
Minimal cognitive load, simple tool patterns

## Scoring System

**100-point scale:**
- **Completion (40pts)**: Task success, compilation, requirements
- **Efficiency (30pts)**: Iterations, tool accuracy, token usage  
- **Quality (20pts)**: Code quality, error handling, best practices
- **Strategy (10pts)**: Planning, error recovery, adaptation

## Results Analysis

Results are automatically saved as JSON files with:
- Execution metrics (time, iterations, tool calls)
- Quality metrics (compilation, test results)
- Detailed logs for analysis

## Adding New Tests

1. **Create baseline files** in `baseline_files/`
2. **Define scenario** in `test_scenarios/scenario_name.md`
3. **Specify success criteria** with measurable outcomes
4. **Run tests** across multiple prompt versions
5. **Analyze results** using comparison tools

## Best Practices

- Run each test 2-3 times for statistical significance
- Test prompt changes incrementally 
- Focus on objective, measurable improvements
- Document hypothesis for each prompt change
- Use controlled environment (same files, same tasks)

## File Structure

```
test_environment/
├── baseline_files/         # Clean test files (reset for each test)
├── test_scenarios/         # Specific tasks with success criteria  
├── prompt_versions/        # Different system prompts to test
├── test_results/          # JSON results and logs
├── run_test.sh           # Main test runner
├── compare_results.sh    # Analysis and comparison tool
└── README.md            # This file
```

## Example Workflow

```bash
# 1. Test baseline performance
./run_test.sh v1_current scenario_1_add_function
./run_test.sh v1_current scenario_2_fix_bug

# 2. Test improved prompt
./run_test.sh v2_structured scenario_1_add_function  
./run_test.sh v2_structured scenario_2_fix_bug

# 3. Compare results
./compare_results.sh scenario_1_add_function
./compare_results.sh scenario_2_fix_bug

# 4. Iterate based on results
```

This framework provides objective, measurable feedback for prompt engineering improvements.