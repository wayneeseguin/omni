#!/bin/bash

echo "=== RUNNING INTEGRATION TEST WITHOUT VERBOSE MODE ==="
echo ""
go test -run TestMultiDestinationLogging ./pkg/omni
echo ""
echo ""

echo "=== RUNNING INTEGRATION TEST WITH VERBOSE MODE ==="
echo ""
go test -v -run TestMultiDestinationLogging ./pkg/omni
echo ""