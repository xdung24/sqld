#!/bin/bash

# Test script for PostgreSQL schema support

echo "Testing PostgreSQL schema support..."

# Test 1: Default public schema
echo "Test 1: Testing public schema (default)..."
echo "Starting sqld with public schema..."
./sqld -u postgres -p changeme -db sqld -h localhost:5432 -type postgres -port 8080 -debug &
SQLD_PID=$!
sleep 2

# Test API call to public schema
echo "Testing GET request to public schema table..."
curl -X GET "http://localhost:8080/data" || echo "Expected: table might not exist in public schema"

# Stop sqld
kill $SQLD_PID
sleep 1

# Test 2: Custom schema
echo ""
echo "Test 2: Testing test_schema..."
echo "Starting sqld with test_schema..."
./sqld -u postgres -p changeme -db sqld -schema test_schema -h localhost:5432 -type postgres -port 8080 -debug &
SQLD_PID=$!
sleep 2

# Test API call to custom schema
echo "Testing GET request to test_schema.products..."
curl -X GET "http://localhost:8080/products"

echo ""
echo "Testing POST request to test_schema.products..."
curl -X POST "http://localhost:8080/products" \
  -H "Content-Type: application/json" \
  -d '{"name": "Test Product", "price": 19.99, "category": "Test"}'

echo ""
echo "Testing GET request again to see new product..."
curl -X GET "http://localhost:8080/products"

# Stop sqld
kill $SQLD_PID

echo ""
echo "Schema tests completed!"
