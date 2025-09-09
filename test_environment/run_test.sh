#!/bin/bash

# Systematic Prompt Testing Runner
# Usage: ./run_test.sh <prompt_version> <scenario> [--debug]

set -e

if [ $# -lt 2 ]; then
    echo "Usage: $0 <prompt_version> <scenario> [--debug]"
    echo "Example: $0 v1_current scenario_1_add_function"
    exit 1
fi

PROMPT_VERSION="$1"
SCENARIO="$2"
DEBUG_MODE=""
if [ "$3" = "--debug" ]; then
    DEBUG_MODE="DEBUG=true"
fi

# Configuration
TEST_DIR="$(pwd)"
BASELINE_DIR="$TEST_DIR/baseline_files"
SCENARIOS_DIR="$TEST_DIR/test_scenarios"
PROMPTS_DIR="$TEST_DIR/prompt_versions"
RESULTS_DIR="$TEST_DIR/test_results"
WORK_DIR="$TEST_DIR/work_${SCENARIO}_${PROMPT_VERSION}_$(date +%s)"

echo "🧪 Running Systematic Prompt Test"
echo "=================================="
echo "Prompt Version: $PROMPT_VERSION"
echo "Scenario: $SCENARIO"
echo "Work Directory: $WORK_DIR"
echo ""

# Create isolated work directory
mkdir -p "$WORK_DIR"
cp -r "$BASELINE_DIR"/* "$WORK_DIR/"

# Read scenario description
SCENARIO_FILE="$SCENARIOS_DIR/${SCENARIO}.md"
if [ ! -f "$SCENARIO_FILE" ]; then
    echo "❌ Scenario file not found: $SCENARIO_FILE"
    exit 1
fi

# Extract task description from scenario
TASK_DESCRIPTION=$(grep -A 10 "## Task Description" "$SCENARIO_FILE" | tail -n +2 | sed '/^## /q' | sed '$d')

echo "📋 Task: $TASK_DESCRIPTION"
echo ""

# Start timing
START_TIME=$(date +%s)

# Change to work directory
cd "$WORK_DIR"

# Run the agent with the specific prompt version
echo "🚀 Starting agent execution..."
echo "=============================="

# Create test execution command
CODER_PATH="/data/data/com.termux/files/usr/var/lib/proot-distro/installed-rootfs/debian/home/alanp/dev/personal/coder/coder"
AGENT_CMD="cd '$WORK_DIR' && $DEBUG_MODE timeout 300 echo '$TASK_DESCRIPTION' | '$CODER_PATH'"

# Execute and capture output
if eval $AGENT_CMD > execution_output.log 2>&1; then
    EXECUTION_SUCCESS=true
    echo "✅ Agent execution completed"
else
    EXECUTION_SUCCESS=false
    echo "⚠️  Agent execution ended (timeout or completion)"
fi

END_TIME=$(date +%s)
EXECUTION_TIME=$((END_TIME - START_TIME))

echo ""
echo "⏱️  Execution time: ${EXECUTION_TIME}s"

# Extract metrics from output
ITERATIONS=$(grep -o "Iteration [0-9]*/" execution_output.log | tail -1 | grep -o "[0-9]*" | head -1 2>/dev/null || echo "0")
TOTAL_TOKENS=$(grep -o "Total cost:" execution_output.log | wc -l 2>/dev/null || echo "0")
TOOL_CALLS=$(grep -c "\[34m\[.*\].*\[0m" execution_output.log 2>/dev/null || echo "0")

# Ensure we have valid numbers
ITERATIONS=${ITERATIONS:-0}
TOTAL_TOKENS=${TOTAL_TOKENS:-0}
TOOL_CALLS=${TOOL_CALLS:-0}

echo "📊 Basic Metrics:"
echo "   Iterations: $ITERATIONS"
echo "   Tool Calls: $TOOL_CALLS"
echo "   Execution Time: ${EXECUTION_TIME}s"

# Run automated checks
echo ""
echo "🔍 Running Automated Checks..."
echo "=============================="

SCORE=0
MAX_SCORE=100

# Check if code compiles
echo -n "Compile Check: "
if go build . >/dev/null 2>&1; then
    echo "✅ PASS (+10 points)"
    SCORE=$((SCORE + 10))
    COMPILE_SUCCESS=true
else
    echo "❌ FAIL (0 points)"
    COMPILE_SUCCESS=false
fi

# Check if tests pass (if test files exist)
echo -n "Test Check: "
if ls *_test.go >/dev/null 2>&1; then
    if go test . >/dev/null 2>&1; then
        echo "✅ PASS (+10 points)"
        SCORE=$((SCORE + 10))
    else
        echo "❌ FAIL (0 points)"
    fi
else
    echo "⊝ N/A (no test files)"
fi

# Efficiency scoring based on iterations
echo -n "Efficiency Check: "
if [ "$ITERATIONS" -le 5 ] && [ "$ITERATIONS" -gt 0 ]; then
    echo "✅ EXCELLENT - $ITERATIONS iterations (+15 points)"
    SCORE=$((SCORE + 15))
elif [ "$ITERATIONS" -le 10 ]; then
    echo "✅ GOOD - $ITERATIONS iterations (+10 points)"
    SCORE=$((SCORE + 10))
elif [ "$ITERATIONS" -le 15 ]; then
    echo "⚠️  OK - $ITERATIONS iterations (+5 points)"
    SCORE=$((SCORE + 5))
else
    echo "❌ POOR - $ITERATIONS iterations (0 points)"
fi

# Save detailed results
RESULT_FILE="$RESULTS_DIR/result_${SCENARIO}_${PROMPT_VERSION}_$(date +%Y%m%d_%H%M%S).json"
cat > "$RESULT_FILE" <<EOF
{
  "test_metadata": {
    "prompt_version": "$PROMPT_VERSION",
    "scenario": "$SCENARIO",
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "work_directory": "$WORK_DIR"
  },
  "execution_metrics": {
    "execution_time_seconds": $EXECUTION_TIME,
    "iterations": $ITERATIONS,
    "tool_calls": $TOOL_CALLS,
    "execution_success": $EXECUTION_SUCCESS
  },
  "quality_metrics": {
    "compiles": $COMPILE_SUCCESS,
    "score": $SCORE,
    "max_score": $MAX_SCORE,
    "percentage": $(( SCORE * 100 / MAX_SCORE ))
  },
  "files_created": $(find . -type f -newer ../baseline_files/README.md | wc -l),
  "files_modified": $(find . -type f -name "*.go" -o -name "*.json" -o -name "*.md" | wc -l)
}
EOF

echo ""
echo "📈 Final Score: $SCORE/$MAX_SCORE ($(( SCORE * 100 / MAX_SCORE ))%)"
echo "📄 Results saved to: $RESULT_FILE"
echo ""
echo "🔍 To review execution details:"
echo "   cat '$WORK_DIR/execution_output.log'"
echo "   ls -la '$WORK_DIR'"
echo ""
echo "✨ Test completed!"