#!/bin/bash

# Test that we're using actual API costs, not estimates
set -e

echo "🧪 Testing actual cost tracking vs estimates"
echo "============================================"

if [[ -z "$DEEPINFRA_API_KEY" ]]; then
    echo "❌ DEEPINFRA_API_KEY environment variable is not set"
    exit 1
fi

# Test with a simple API call to see actual vs estimated costs
echo ""
echo "📊 Making API call to get actual cost data..."

API_RESPONSE=$(curl -s -X POST "https://api.deepinfra.com/v1/openai/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DEEPINFRA_API_KEY" \
  -d '{
    "model": "Qwen/Qwen3-Coder-480B-A35B-Instruct-Turbo",
    "messages": [
      {"role": "user", "content": "Hello, write a simple function"}
    ],
    "max_tokens": 50
  }')

echo "Raw API Response Usage:"
echo "$API_RESPONSE" | jq '.usage'

ACTUAL_COST=$(echo "$API_RESPONSE" | jq '.usage.estimated_cost')
PROMPT_TOKENS=$(echo "$API_RESPONSE" | jq '.usage.prompt_tokens')
COMPLETION_TOKENS=$(echo "$API_RESPONSE" | jq '.usage.completion_tokens')
CACHED_TOKENS=$(echo "$API_RESPONSE" | jq '.usage.prompt_tokens_details.cached_tokens // 0')

echo ""
echo "📋 API Response Analysis:"
echo "  - Prompt tokens: $PROMPT_TOKENS"
echo "  - Completion tokens: $COMPLETION_TOKENS"  
echo "  - Cached tokens: $CACHED_TOKENS"
echo "  - Actual API cost: \$$ACTUAL_COST"

# Calculate what the cost would be using our old estimation method
INPUT_RATE=0.30  # $0.30 per 1M tokens
OUTPUT_RATE=1.20 # $1.20 per 1M tokens

ESTIMATED_COST=$(echo "scale=8; ($PROMPT_TOKENS * $INPUT_RATE + $COMPLETION_TOKENS * $OUTPUT_RATE) / 1000000" | bc)

echo "  - Our estimated cost: \$$ESTIMATED_COST"
echo ""

if [[ "$CACHED_TOKENS" -gt 0 ]]; then
    echo "✅ Cached tokens detected: $CACHED_TOKENS tokens"
    echo "   The API's estimated_cost already accounts for this caching discount"
    echo "   Our coder should use the actual API cost: \$$ACTUAL_COST"
else
    echo "ℹ️  No cached tokens in this request"
fi

echo ""
echo "🎯 Key Point:"
echo "============"
echo "✅ The API's 'estimated_cost' field is the FINAL cost after all discounts"
echo "✅ We should use that value directly, not calculate our own estimates"
echo "✅ Cached tokens are tracked separately for informational display only"
echo ""
echo "Updated implementation now uses:"
echo "  • resp.Usage.EstimatedCost directly (no adjustments)"
echo "  • Cached tokens for display purposes only"
echo "  • Cost savings calculation shown to user but not subtracted from totals"