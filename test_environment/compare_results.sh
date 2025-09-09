#!/bin/bash

# Compare results across different prompt versions
# Usage: ./compare_results.sh <scenario>

if [ $# -ne 1 ]; then
    echo "Usage: $0 <scenario>"
    echo "Example: $0 scenario_1_add_function"
    exit 1
fi

SCENARIO="$1"
RESULTS_DIR="./test_results"

echo "📊 Comparison Report for $SCENARIO"
echo "================================="
echo ""

# Find all result files for this scenario
RESULT_FILES=($(ls "$RESULTS_DIR"/result_"$SCENARIO"_*.json 2>/dev/null || echo ""))

if [ ${#RESULT_FILES[@]} -eq 0 ]; then
    echo "❌ No test results found for scenario: $SCENARIO"
    echo "Run some tests first with: ./run_test.sh <prompt_version> $SCENARIO"
    exit 1
fi

echo "Found ${#RESULT_FILES[@]} test results:"
echo ""

# Create comparison table header
printf "%-15s %-8s %-10s %-8s %-8s %-10s\n" "Prompt Version" "Score" "Iterations" "Time(s)" "Compile" "Tool Calls"
printf "%-15s %-8s %-10s %-8s %-8s %-10s\n" "===============" "======" "==========" "=======" "=======" "=========="

# Parse and display results
for result_file in "${RESULT_FILES[@]}"; do
    # Extract data from JSON
    prompt_version=$(jq -r '.test_metadata.prompt_version' "$result_file")
    score=$(jq -r '.quality_metrics.percentage' "$result_file")
    iterations=$(jq -r '.execution_metrics.iterations' "$result_file")
    time=$(jq -r '.execution_metrics.execution_time_seconds' "$result_file")
    compiles=$(jq -r '.quality_metrics.compiles' "$result_file")
    tool_calls=$(jq -r '.execution_metrics.tool_calls' "$result_file")
    
    # Format compile status
    if [ "$compiles" = "true" ]; then
        compile_status="✅"
    else
        compile_status="❌"
    fi
    
    printf "%-15s %-8s %-10s %-8s %-8s %-10s\n" "$prompt_version" "${score}%" "$iterations" "$time" "$compile_status" "$tool_calls"
done

echo ""
echo "📈 Analysis:"

# Find best performing version
BEST_FILE=$(jq -s 'max_by(.quality_metrics.percentage)' "${RESULT_FILES[@]}")
BEST_PROMPT=$(echo "$BEST_FILE" | jq -r '.test_metadata.prompt_version')
BEST_SCORE=$(echo "$BEST_FILE" | jq -r '.quality_metrics.percentage')

echo "🏆 Best performing prompt: $BEST_PROMPT (${BEST_SCORE}%)"

# Calculate averages
TOTAL_SCORE=0
TOTAL_ITERATIONS=0
TOTAL_TIME=0
COMPILE_SUCCESS=0

for result_file in "${RESULT_FILES[@]}"; do
    score=$(jq -r '.quality_metrics.percentage' "$result_file")
    iterations=$(jq -r '.execution_metrics.iterations' "$result_file")
    time=$(jq -r '.execution_metrics.execution_time_seconds' "$result_file")
    compiles=$(jq -r '.quality_metrics.compiles' "$result_file")
    
    TOTAL_SCORE=$((TOTAL_SCORE + score))
    TOTAL_ITERATIONS=$((TOTAL_ITERATIONS + iterations))
    TOTAL_TIME=$((TOTAL_TIME + time))
    
    if [ "$compiles" = "true" ]; then
        COMPILE_SUCCESS=$((COMPILE_SUCCESS + 1))
    fi
done

NUM_TESTS=${#RESULT_FILES[@]}
AVG_SCORE=$((TOTAL_SCORE / NUM_TESTS))
AVG_ITERATIONS=$((TOTAL_ITERATIONS / NUM_TESTS))
AVG_TIME=$((TOTAL_TIME / NUM_TESTS))
COMPILE_RATE=$(echo "scale=1; $COMPILE_SUCCESS * 100 / $NUM_TESTS" | bc -l)

echo "📊 Averages across $NUM_TESTS tests:"
echo "   Average Score: ${AVG_SCORE}%"
echo "   Average Iterations: $AVG_ITERATIONS"
echo "   Average Time: ${AVG_TIME}s"
echo "   Compile Success Rate: ${COMPILE_RATE}%"

echo ""
echo "💡 Recommendations:"

if [ "$AVG_SCORE" -lt 70 ]; then
    echo "❌ Overall performance is poor (<70%). Consider major prompt redesign."
elif [ "$AVG_SCORE" -lt 85 ]; then
    echo "⚠️  Performance is okay but needs improvement. Focus on error handling."
else
    echo "✅ Good performance! Focus on efficiency and edge case handling."
fi

if [ "$AVG_ITERATIONS" -gt 10 ]; then
    echo "⚠️  High iteration count suggests inefficient approach. Improve planning."
fi

if [ $(echo "$COMPILE_RATE < 80" | bc -l) -eq 1 ]; then
    echo "❌ Low compile success rate. Focus on code quality and syntax."
fi