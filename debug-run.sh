#!/bin/bash
# Run the IDEO protocol with debug logging to see what messages go to each LLM call
# Usage: ./debug-run.sh "Your topic here"

export MEANWHILE_DEBUG_LLM=1
export MEANWHILE_MODEL=gpt-4o-mini

TOPIC="${1:-How can we improve onboarding?}"

echo "Running IDEO brainstorm with debug LLM logging..."
echo "Topic: $TOPIC"
echo ""
echo "Output will show the exact messages sent to each LLM call."
echo "Look for: ╔══════════════════════ boxes showing each request"
echo ""

go run ./examples/26-protocol-brainstorm-ideo/main.go "$TOPIC" 2>&1 | tee debug-output.log
