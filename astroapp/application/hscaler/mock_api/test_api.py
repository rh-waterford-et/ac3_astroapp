#!/usr/bin/env python3
"""
Test script for the Model Prediction API

Tests the /health and /predict endpoints to ensure the API is working correctly.
"""

import requests
import json
import sys

def test_health(base_url):
    """Test the health endpoint."""
    print("Testing /health endpoint...")
    try:
        response = requests.get(f"{base_url}/health")
        print(f"Status Code: {response.status_code}")
        print(f"Response: {response.json()}")
        
        if response.status_code == 200 and response.json().get('status') == 'healthy':
            print("✅ Health check PASSED\n")
            return True
        else:
            print("❌ Health check FAILED\n")
            return False
    except Exception as e:
        print(f"❌ Health check ERROR: {e}\n")
        return False

def test_predict(base_url, test_cases):
    """Test the predict endpoint with various test cases."""
    print("Testing /predict endpoint...")
    
    all_passed = True
    for i, test_case in enumerate(test_cases, 1):
        print(f"\nTest Case {i}: {test_case}")
        try:
            response = requests.post(
                f"{base_url}/predict",
                json=test_case,
                headers={'Content-Type': 'application/json'}
            )
            print(f"Status Code: {response.status_code}")
            print(f"Response: {json.dumps(response.json(), indent=2)}")
            
            if response.status_code == 200:
                data = response.json()
                if 'predicted_job_time' in data:
                    print(f"✅ Prediction received: {data['predicted_job_time']} seconds")
                else:
                    print("❌ Response missing 'predicted_job_time' field")
                    all_passed = False
            else:
                print(f"❌ Prediction failed with status {response.status_code}")
                all_passed = False
                
        except Exception as e:
            print(f"❌ Prediction ERROR: {e}")
            all_passed = False
    
    return all_passed

def test_error_handling(base_url):
    """Test error handling with invalid requests."""
    print("\n\nTesting error handling...")
    
    # Test case 1: Missing required field
    print("\n1. Testing missing required field:")
    try:
        response = requests.post(
            f"{base_url}/predict",
            json={"num_processors": 2, "job_size": 100},
            headers={'Content-Type': 'application/json'}
        )
        print(f"Status Code: {response.status_code}")
        print(f"Response: {json.dumps(response.json(), indent=2)}")
        if response.status_code == 400:
            print("✅ Correctly returned 400 for missing field")
        else:
            print("❌ Did not return 400 for missing field")
    except Exception as e:
        print(f"❌ ERROR: {e}")
    
    # Test case 2: No JSON data
    print("\n2. Testing no JSON data:")
    try:
        response = requests.post(f"{base_url}/predict")
        print(f"Status Code: {response.status_code}")
        print(f"Response: {json.dumps(response.json(), indent=2)}")
        if response.status_code == 400:
            print("✅ Correctly returned 400 for no JSON data")
        else:
            print("❌ Did not return 400 for no JSON data")
    except Exception as e:
        print(f"❌ ERROR: {e}")

if __name__ == '__main__':
    # Default to localhost
    base_url = "http://localhost:5000"
    
    if len(sys.argv) > 1:
        base_url = sys.argv[1]
    
    print(f"Testing Model Prediction API at {base_url}")
    print("=" * 60)
    
    # Test health endpoint
    health_ok = test_health(base_url)
    
    if not health_ok:
        print("\n⚠️  Health check failed. Make sure the API is running.")
        sys.exit(1)
    
    # Test cases for prediction
    test_cases = [
        {
            "num_processors": 1,
            "job_size": 100,
            "queue_len": 5
        },
        {
            "num_processors": 2,
            "job_size": 200,
            "queue_len": 10
        },
        {
            "num_processors": 4,
            "job_size": 500,
            "queue_len": 20
        },
        {
            "num_processors": 1,
            "job_size": 50,
            "queue_len": 0
        }
    ]
    
    # Run prediction tests
    predict_ok = test_predict(base_url, test_cases)
    
    # Test error handling
    test_error_handling(base_url)
    
    print("\n" + "=" * 60)
    if health_ok and predict_ok:
        print("✅ All tests PASSED!")
        sys.exit(0)
    else:
        print("❌ Some tests FAILED!")
        sys.exit(1)


